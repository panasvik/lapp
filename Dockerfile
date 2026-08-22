# === Этап 1: Сборка ===
FROM golang:1.26-alpine AS builder

WORKDIR /app

# 1. Устанавливаем системные зависимости для CGO (sqlite3 и bimg)
RUN apk add --no-cache gcc musl-dev pkgconf vips-dev

# 2. Копируем модули и скачиваем зависимости
COPY go.mod go.sum ./
RUN go mod download

# 3. Копируем весь остальной код
COPY . .

# 4. ВАЖНО: Включаем CGO и указываем правильный путь (cmd/app)
RUN CGO_ENABLED=1 GOOS=linux go build -o /my-app ./cmd/app

# === Этап 2: Финальный образ ===
FROM alpine:latest

WORKDIR /

# 5. Устанавливаем библиотеку vips, чтобы bimg работал при запуске
RUN apk add --no-cache vips tzdata

# Копируем готовый бинарник
COPY --from=builder /my-app /my-app

COPY --from=builder /app/loveApp.db /loveApp.db

# Если сервер использует порт (например, 8080), укажите его:
EXPOSE 8080

CMD ["/my-app"]