#!/bin/sh
set -x  # print each command for debugging

# Start Litestream replication in background, log to file
litestream replicate -config /app/litestream.yml > /app/litestream.log 2>&1 &

# Wait a moment for litestream to initialize (optional)
sleep 2

# Start PocketBase on the port Render provides (default 8080)
export PORT=${PORT:-8080}
exec ./pocketbase serve --http=0.0.0.0:${PORT}
