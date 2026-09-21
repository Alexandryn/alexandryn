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
RUN addgroup -S app && adduser -S app -G app \
    && mkdir -p /home/app/.config/alexandryn \
    && chown -R app:app /home/app/.config
# /home/app/.config/alexandryn is created here, owned by app, because
# docker-compose.yml mounts the app-data volume on it: Docker gives a new named
# volume the ownership of the image's directory, and a directory the image does
# not have makes a root-owned volume the app user cannot write its
# credential key into (the server then crash-loops).
COPY --from=build /out/alexandryn-server /usr/local/bin/alexandryn-server
USER app

# Healthcheck queries /readyz on the container's own interface, not its
# loopback: BIND_ADDRESS may be a fixed private address rather than
# 127.0.0.1 (docker-compose.yml's default profile binds one, since a
# published port or a sibling container reaches this container's real
# interface, never its loopback) — `hostname -i` reports that same
# address from inside the container regardless of which one BIND_ADDRESS
# ends up being, so this works for both that case and a plain `docker
# run` with a loopback bind. BIND_ADDRESS must be set to a fixed port
# when running the container because the server's default is an
# ephemeral port.
HEALTHCHECK --interval=5s --timeout=3s --retries=5 \
  CMD wget -qO- "http://$(hostname -i):8080/readyz" || exit 1

ENTRYPOINT ["/usr/local/bin/alexandryn-server"]
