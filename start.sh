#!/bin/sh
set -x

export GOMEMLIMIT=128MiB
export GOGC=20

# Restore database from iDrive if local DB doesn't exist or is empty
if [ ! -s /app/pb_data/data.db ]; then
  echo "Local database missing, restoring from iDrive..."
  litestream restore -config /app/litestream.yml -o /app/pb_data/data.db -debug
fi

# Start Litestream replication in background with debug
litestream replicate -config /app/litestream.yml -debug > /app/litestream.log 2>&1 &

sleep 2

# Start PocketBase on Render's provided port
export PORT=${PORT:-8080}
exec ./pocketbase serve --http=0.0.0.0:${PORT}
