#!/bin/sh
set -eu

export GOMEMLIMIT=128MiB
export GOGC=20

DATA_DIR=/app/pb_data
DB_PATH="$DATA_DIR/data.db"
LITESTREAM_CONFIG=/app/litestream.yml

mkdir -p "$DATA_DIR"

db_size() {
  stat -c%s "$DB_PATH" 2>/dev/null || printf 'missing'
}

echo "[startup] $(date -u '+%Y-%m-%dT%H:%M:%SZ') — ValleyVista backend starting"
echo "[startup] pb_data exists: $( [ -d "$DATA_DIR" ] && printf yes || printf no )"
echo "[startup] data.db size before restore: $(db_size) bytes"

# Always attempt restore. Litestream's -if-db-not-exists flag makes this a
# no-op when a usable local database is already present.
echo "[startup] Attempting Litestream restore..."
if litestream restore -config "$LITESTREAM_CONFIG" -if-db-not-exists -o "$DB_PATH" "$DB_PATH"; then
  echo "[startup] Litestream restore completed"
else
  echo "[startup] Litestream restore returned non-zero; continuing with local data"
fi
echo "[startup] data.db size after restore: $(db_size) bytes"

BACKUP_CONFIGURED=1
for required_var in IDRIVE_BUCKET IDRIVE_REGION IDRIVE_ACCESS_KEY_ID IDRIVE_SECRET_ACCESS_KEY; do
  case "$required_var" in
    IDRIVE_BUCKET) value=${IDRIVE_BUCKET:-} ;;
    IDRIVE_REGION) value=${IDRIVE_REGION:-} ;;
    IDRIVE_ACCESS_KEY_ID) value=${IDRIVE_ACCESS_KEY_ID:-} ;;
    IDRIVE_SECRET_ACCESS_KEY) value=${IDRIVE_SECRET_ACCESS_KEY:-} ;;
  esac
  if [ -z "$value" ]; then
    echo "[startup] WARNING: $required_var is not set; Litestream replication is disabled"
    BACKUP_CONFIGURED=0
  fi
done

LITESTREAM_PID=
start_replication() {
  if [ "$BACKUP_CONFIGURED" -eq 0 ]; then
    return 0
  fi

  echo "[startup] Starting Litestream replication with verbose logging..."
  # Keep output on Render's log stream. The supervisor below restarts the
  # process if iDrive or the network temporarily takes it down.
  litestream replicate -config "$LITESTREAM_CONFIG" -v &
  LITESTREAM_PID=$!
  echo "[startup] Litestream PID: $LITESTREAM_PID"
}

start_replication

export PORT=${PORT:-8080}
echo "[startup] Starting PocketBase on port $PORT"
./pocketbase serve --http=0.0.0.0:"$PORT" &
POCKETBASE_PID=$!

cleanup() {
  trap - INT TERM EXIT
  if [ -n "${LITESTREAM_PID:-}" ] && kill -0 "$LITESTREAM_PID" 2>/dev/null; then
    kill "$LITESTREAM_PID" 2>/dev/null || true
  fi
  if kill -0 "$POCKETBASE_PID" 2>/dev/null; then
    kill "$POCKETBASE_PID" 2>/dev/null || true
  fi
}
trap cleanup INT TERM EXIT

# Keep the database service available while automatically recovering
# replication after a transient iDrive/network/process failure.
while kill -0 "$POCKETBASE_PID" 2>/dev/null; do
  if [ "$BACKUP_CONFIGURED" -eq 1 ] &&
     [ -n "${LITESTREAM_PID:-}" ] &&
     ! kill -0 "$LITESTREAM_PID" 2>/dev/null; then
    echo "[startup] Litestream exited; restarting replication..."
    start_replication
  fi
  sleep 5
done

wait "$POCKETBASE_PID"
