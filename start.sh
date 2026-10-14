#!/bin/sh
set -x

export GOMEMLIMIT=128MiB
export GOGC=20

# 1. Restore DB from Litestream if it's missing
if [ ! -s /app/pb_data/data.db ]; then
  echo "[startup] Restoring DB from Litestream..."
  litestream restore -config /app/litestream.yml -if-db-not-exists -o /app/pb_data/data.db /app/pb_data/data.db || true
fi

# 2. Start Litestream replication with a supervisor loop
(
  while true; do
    echo "[supervisor] Starting Litestream replicate..."
    litestream replicate -config /app/litestream.yml -v
    echo "[supervisor] Litestream died. Restarting in 5s..."
    sleep 5
  done
) &

# 3. Start PocketBase
export PORT=${PORT:-8080}
echo "[startup] Starting PocketBase on $PORT"
exec ./pocketbase serve --http=0.0.0.0:${PORT}
