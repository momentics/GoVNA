# Этап 1: Сборка приложения
FROM golang:1.21-alpine AS build

WORKDIR /app

# Копируем go.mod и go.sum и загружаем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /govna-server ./cmd/server

# Этап 2: Создание минимального образа
FROM alpine:latest

# Добавляем корневые сертификаты
RUN apk --no-cache add ca-certificates

# Устанавливаем рабочую директорию
WORKDIR /root/

# Копируем бинарный файл из этапа сборки
COPY --from=build /govna-server .

# Открываем порт
EXPOSE 8080

# Запускаем приложение
ENTRYPOINT ["./govna-server"]
