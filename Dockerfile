# --- build stage ---
FROM golang:1.26-alpine AS build

WORKDIR /src

# Кэш зависимостей: сначала только модульные файлы.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO выключен → статичный бинарник для scratch/distroless.
# -trimpath убирает пути сборки, -ldflags "-s -w" снимает debug-инфу.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w" \
    -o /out/demo-app \
    ./cmd/demo-app

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/demo-app /demo-app

EXPOSE 8080

# Распространённые умолчания. Переопределяются через docker run -e / compose.
ENV HTTP_ADDR=:8080 \
    REDIS_ADDR=redis:6379 \
    DOC_ID=demo

USER nonroot:nonroot
ENTRYPOINT ["/demo-app"]
