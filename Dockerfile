# Многоступенчатая сборка с кэшированием
FROM golang:1.24-alpine AS builder

# Устанавливаем необходимые пакеты для сборки
RUN apk add --no-cache git ca-certificates tzdata

# Создаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости (этот слой будет кэшироваться)
RUN go mod download && go mod verify

# Копируем исходный код
COPY . .

# Собираем приложение с оптимизацией
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" \
    -o server ./cmd/server

# Минимальный production образ
FROM alpine:3.19

# Устанавливаем необходимые пакеты
RUN apk --no-cache add \
    ca-certificates \
    tzdata \
    curl \
    && update-ca-certificates

# Создаем пользователя без привилегий
RUN addgroup -S appgroup && \
    adduser -S -G appgroup -s /sbin/nologin appuser

# Создаем необходимые директории
RUN mkdir -p /app/uploads /app/config && \
    chown -R appuser:appgroup /app

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем собранный бинарник
COPY --from=builder /app/server ./server

# Копируем конфигурацию (если есть)
COPY --from=builder /app/config ./config

# Устанавливаем права доступа
RUN chown -R appuser:appgroup /app && \
    chmod +x /app/server

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8080

# Запуск приложения
ENTRYPOINT ["./server"]