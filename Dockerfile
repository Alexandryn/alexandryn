# backend-test-harness.md FR-10, deployment-container-packaging.md FR-1/FR-2/FR-3.
# Multi-stage: build stage compiles web/ (once phase 04 lands) then the Go
# binary, ADR 0008's ordering; runtime stage ships only the binary.

FROM golang:1.26-alpine AS build

WORKDIR /src
COPY . .

# No web/ directory exists yet (phase 04) — nodejs/npm are installed only
# inside this conditional, so today's build (and every CI run until
# phase 04 lands) never pays for them. The moment web/ exists, this same
# line installs what it needs and builds it — no Dockerfile edit
# required then, the same "build against what's committed, swap later"
# pattern D2 used for internal/transport/http/webdist's placeholder
# embed (deployment-container-packaging.md FR-1).
RUN if [ -d web ]; then apk add --no-cache nodejs npm && cd web && npm ci && npm run build; fi
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
