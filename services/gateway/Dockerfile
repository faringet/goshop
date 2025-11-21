FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/gateway ./services/gateway/cmd/gateway

# fetch grpc_health_probe
FROM alpine:3.20 AS probe
ARG PROBE_VERSION=v0.4.26
ARG TARGETARCH
RUN apk add --no-cache ca-certificates wget \
 && wget -O /grpc_health_probe "https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${PROBE_VERSION}/grpc_health_probe-linux-${TARGETARCH}" \
 && chmod +x /grpc_health_probe

# runtime
FROM alpine:3.20 AS gateway-app
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/gateway /app/app
COPY services/gateway/config /app/config
# кладём probe в PATH
COPY --from=probe /grpc_health_probe /usr/local/bin/grpc_health_probe
ENV APP_ENV=docker
USER app
ENTRYPOINT ["/app/app"]
