FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o pocketbase .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/pocketbase .

# Create data directory (Render will mount a persistent disk here)
RUN mkdir -p /app/pb_data

EXPOSE 8080

CMD ["./pocketbase", "serve", "--http=0.0.0.0:8080"]
