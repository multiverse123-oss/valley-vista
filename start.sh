#!/bin/sh
set -e

# Start Litestream replication (background)
litestream replicate -config /app/litestream.yml &

# Start PocketBase (foreground)
exec ./pocketbase serve --http=0.0.0.0:8080
