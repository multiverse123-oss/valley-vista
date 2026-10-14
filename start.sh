#!/bin/sh
set -x

export GOMEMLIMIT=128MiB
export GOGC=20

# Restore DB from Litestream if it's missing
if [ ! -s /app/pb_data/data.db ]; then
  echo "[startup] Restoring DB from Litestream..."
  litestream restore -config /app/litestream.yml -if-db-not-exists -o /app/pb_data/data.db /app/pb_data/data.db || true
fi

# Supervisor loop keeps Litestream running — NO -v flag
(
  while true; do
    echo "[supervisor] Starting Litestream replicate..."
    litestream replicate -config /app/litestream.yml
    echo "[supervisor] Litestream exited. Restarting in 5s..."
    sleep 5
  done
) &

# Start PocketBase
export PORT=${PORT:-8080}
echo "[startup] Starting PocketBase on $PORT"
exec ./pocketbase serve --http=0.0.0.0:${PORT}
