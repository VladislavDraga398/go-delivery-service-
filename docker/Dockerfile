# Build stage
FROM golang:1.23-alpine AS builder

# Установка зависимостей для сборки
RUN apk add --no-cache git ca-certificates

# Создание рабочей директории
WORKDIR /build

# Копирование go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./

# Загрузка зависимостей
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o delivery-server ./cmd/server

# Production stage
FROM alpine:latest

# Установка необходимых утилит
RUN apk --no-cache add ca-certificates tzdata

# Создание пользователя для безопасности
RUN adduser -D -s /bin/sh appuser

# Создание рабочей директории
WORKDIR /app

# Создание директории для логов
RUN mkdir -p /app/logs && chown appuser:appuser /app/logs

# Копирование бинарного файла из builder stage
COPY --from=builder /build/delivery-server .

# Изменение владельца файлов
RUN chown appuser:appuser delivery-server

# Переключение на пользователя appuser
USER appuser

# Экспорт порта
EXPOSE 8080

# Healthcheck
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Запуск приложения
CMD ["./delivery-server"] 