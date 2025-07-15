FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" \
    -o server ./cmd/server

FROM alpine:3.19

RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && update-ca-certificates

RUN addgroup -S appgroup && \
    adduser -S -G appgroup -s /sbin/nologin appuser

RUN mkdir -p /app/uploads /app/config && \
    chown -R appuser:appgroup /app

WORKDIR /app

COPY --from=builder /app/server ./server

COPY --from=builder /app/config ./config

RUN chown -R appuser:appgroup /app && \
    chmod +x /app/server

USER appuser

EXPOSE 8080

ENTRYPOINT ["./server"]