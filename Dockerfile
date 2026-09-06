FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o pocketbase .

# Download Litestream binary
FROM alpine:latest AS litestream

ARG LITESTREAM_VERSION="0.3.13"

RUN apk --no-cache add ca-certificates wget \
    && wget -qO /tmp/litestream.tar.gz "https://github.com/benbjohnson/litestream/releases/download/v${LITESTREAM_VERSION}/litestream-v${LITESTREAM_VERSION}-linux-amd64.tar.gz" \
    && tar -xzf /tmp/litestream.tar.gz -C /tmp \
    && mv /tmp/litestream /usr/local/bin/litestream \
    && chmod +x /usr/local/bin/litestream

# Final image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy PocketBase binary
COPY --from=builder /app/pocketbase .

# Copy Litestream binary
COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream

# Copy configuration and start script
COPY litestream.yml .
COPY start.sh .

# Make start.sh executable
RUN chmod +x start.sh

# Create data directory
RUN mkdir -p /app/pb_data

EXPOSE 8080

CMD ["./start.sh"]
