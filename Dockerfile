# Multi-stage container build: compiles frontend assets before the Go binary,
# then copies only the static server binary into a minimal runtime image.

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY . .

# web/ is an npm workspace member (package.json at repo root),
# so `npm ci` runs at /src and the web build is invoked with `-w web`.
# nodejs/npm are installed only inside this conditional so a Go-only
# checkout never pays for them.
RUN if [ -d web ]; then apk add --no-cache nodejs npm && npm ci && npm run -w web build; fi
RUN CGO_ENABLED=0 go build -o /out/alexandryn-server ./cmd/server

FROM alpine:3.22 AS runtime

# Non-root runtime user created explicitly rather than relying on base image default.
RUN addgroup -S app && adduser -S app -G app
COPY --from=build /out/alexandryn-server /usr/local/bin/alexandryn-server
USER app

# Healthcheck queries /readyz over loopback using busybox wget.
# Note: BIND_ADDRESS must be set to a fixed port when running the container
# because the server's default is an ephemeral port.
HEALTHCHECK --interval=5s --timeout=3s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/readyz || exit 1

ENTRYPOINT ["/usr/local/bin/alexandryn-server"]
