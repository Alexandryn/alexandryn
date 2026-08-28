# backend-test-harness.md FR-10, deployment-container-packaging.md FR-1/FR-2/FR-3.
# Multi-stage: build stage compiles web/ (once phase 04 lands) then the Go
# binary, ADR 0008's ordering; runtime stage ships only the binary.

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY . .

# web/ is an npm workspace member (ADR 0008, package.json at repo root),
# so `npm ci` runs at /src and the web build is invoked with `-w web`.
# nodejs/npm are installed only inside this conditional so a Go-only
# checkout never pays for them.
RUN if [ -d web ]; then apk add --no-cache nodejs npm && npm ci && npm run -w web build; fi
RUN CGO_ENABLED=0 go build -o /out/alexandryn-server ./cmd/server

FROM alpine:3.22 AS runtime

# Non-root runtime user, created explicitly rather than relying on a base
# image default (deployment-container-packaging.md FR-2).
RUN addgroup -S app && adduser -S app -G app
COPY --from=build /out/alexandryn-server /usr/local/bin/alexandryn-server
USER app

# FR-3: /readyz over loopback, inside this container's own namespace —
# wget is already present via busybox, no extra install. BIND_ADDRESS is
# set by docker-compose.yml to a fixed port (deployment-container-
# packaging.md's own T27-D1); the application's own default is an
# ephemeral port, unusable as a HEALTHCHECK target.
HEALTHCHECK --interval=5s --timeout=3s --retries=5 \
  CMD wget -qO- http://127.0.0.1:8080/readyz || exit 1

ENTRYPOINT ["/usr/local/bin/alexandryn-server"]
