# --- build stage ---
FROM golang:1.26-alpine AS build

WORKDIR /src

# кэш зависимостей
COPY go.mod go.sum ./
RUN go mod download

COPY . .

#  history-service
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w" \
    -o /out/history-service \
    ./cmd/history-service

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/history-service /history-service

EXPOSE 8083

ENV HTTP_ADDR=:8083 \
    REDIS_ADDR=redis:6379 \
    DB_DSN=postgres://crdt:crdt_password@analytics-db:5432/crdt_analytics?sslmode=disable

USER nonroot:nonroot
ENTRYPOINT ["/history-service"]
