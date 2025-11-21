FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# приложение
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/outboxer ./services/outboxer/cmd/outboxer

# runtime
FROM alpine:3.20 AS outboxer-app
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/outboxer /app/app
COPY services/outboxer/config /app/config
ENV APP_ENV=docker
USER app
ENTRYPOINT ["/app/app"]
