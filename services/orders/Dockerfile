FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# приложение и мигратор
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/orders-app     ./services/orders/cmd/orders
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/orders-migrate ./services/orders/cmd/migrate

# ----- runtime: app -----
FROM alpine:3.20 AS orders-app
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/orders-app /app/app
COPY services/orders/config /app/config
ENV APP_ENV=docker
USER app
ENTRYPOINT ["/app/app"]

# ----- runtime: migrate -----
FROM alpine:3.20 AS orders-migrate
WORKDIR /app
COPY --from=build /out/orders-migrate /usr/local/bin/migrate
COPY services/orders/config /app/config
COPY services/orders/cmd/migrate/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENV APP_ENV=docker
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
