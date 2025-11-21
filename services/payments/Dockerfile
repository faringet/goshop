FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# приложение и мигратор
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/payments-app     ./services/payments/cmd/payments
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/payments-migrate ./services/payments/cmd/migrate

# ----- runtime: app -----
FROM alpine:3.20 AS payments-app
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/payments-app /app/app
COPY services/payments/config /app/config
ENV APP_ENV=docker
USER app
ENTRYPOINT ["/app/app"]

# ----- runtime: migrate -----
FROM alpine:3.20 AS payments-migrate
WORKDIR /app
COPY --from=build /out/payments-migrate /usr/local/bin/migrate
COPY services/payments/config /app/config
COPY services/payments/cmd/migrate/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENV APP_ENV=docker
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
