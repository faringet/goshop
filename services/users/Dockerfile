FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# приложение и мигратор
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/users-app     ./services/users/cmd/users
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/users-migrate ./services/users/cmd/migrate

# ----- runtime: app -----
FROM alpine:3.20 AS users-app
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/users-app /app/app
COPY services/users/config /app/config
ENV APP_ENV=docker
USER app
ENTRYPOINT ["/app/app"]

# ----- runtime: migrate
FROM alpine:3.20 AS users-migrate
WORKDIR /app
COPY --from=build /out/users-migrate /usr/local/bin/migrate
COPY services/users/config /app/config
COPY services/users/cmd/migrate/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh
ENV APP_ENV=docker
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]