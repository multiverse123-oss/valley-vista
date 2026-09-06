#!/bin/sh
set -x

# Ultra-low memory settings to fit 512MB
export GOMEMLIMIT=128MiB
export GOGC=20

# Start Litestream replication in background
litestream replicate -config /app/litestream.yml > /app/litestream.log 2>&1 &

# Wait briefly
sleep 2

# Start PocketBase on Render's provided port
export PORT=${PORT:-8080}
exec ./pocketbase serve --http=0.0.0.0:${PORT}
