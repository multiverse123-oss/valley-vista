#!/bin/sh
set -x

# Aggressive garbage collection to stay under Render's memory limit
export GOMEMLIMIT=256MiB
export GOGC=50

# Start Litestream replication in background
litestream replicate -config /app/litestream.yml > /app/litestream.log 2>&1 &

# Wait briefly to allow Litestream to attach
sleep 2

# Start PocketBase on Render's provided port
export PORT=${PORT:-8080}
exec ./pocketbase serve --http=0.0.0.0:${PORT}
