# --- build stage ---
FROM golang:1.26-alpine AS build

WORKDIR /src

# Кэш зависимостей
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Собираем статичный бинарник для analytics-service
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w" \
    -o /out/analytics-service \
    ./cmd/analytics-service

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/analytics-service /analytics-service

EXPOSE 8082

ENV HTTP_ADDR=:8082 \
    REDIS_ADDR=redis:6379 \
    DB_DSN=postgres://crdt:crdt_password@analytics-db:5432/crdt_analytics?sslmode=disable

USER nonroot:nonroot
ENTRYPOINT ["/analytics-service"]
