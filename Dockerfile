FROM golang:1.24-alpine AS builder

RUN apk add --no-cache tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -S appgroup && adduser -S -G appgroup -s /sbin/nologin appuser

WORKDIR /app

COPY --from=builder /app/server ./server
COPY --from=builder /app/config ./config

RUN mkdir -p /app/uploads && \
    chown -R appuser:appgroup /app && \
    chmod +x /app/server

USER appuser

EXPOSE 8080

ENTRYPOINT ["./server"]
