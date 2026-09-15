# API contract

Alexandryn's HTTP API is specified in [`api/openapi.yaml`](api/openapi.yaml),
an OpenAPI 3.1 document. That file is the source of truth for every request
and response shape the Go server exposes — it is hand-authored, not
generated from the server's Go code, so a change to the API is a change to
the contract file first.

## Scope

The contract covers the server's own HTTP surface: the REST API consumed by
the Electron renderer and by any browser on the same network once network
access is enabled. It does not cover the Electron preload/IPC boundary
(that's a separate, non-HTTP surface) and it does not cover realtime or
event-stream schemas, since the API is request/response only.

## Versioning

The API is versioned by path (`/api/v1/...`). The version exists for one
concrete case: a browser tab that stayed open across a host update keeps
running its already-loaded frontend, so a breaking change needs a new path
prefix to avoid breaking that tab silently. It is not versioned the way a
public API with independently-deployed clients is — Alexandryn's web UI is
served fresh by the Go server on every new page load, so there is normally
no client/server skew to manage.

`/healthz` and `/readyz` sit outside the versioned surface; they predate it
and never change shape.

## Authentication

Every route requires a bearer token (`Authorization: Bearer <token>`)
except an explicit, short allowlist: `/healthz`, `/readyz`, the account
setup and login flow, token refresh and logout, password reset request and
confirmation, TOTP verification during login, device pairing verification,
and the anonymous capability probe (`GET /api/bootstrap`). Everything else
— including every reading-data endpoint (progress, bookmarks, highlights,
preferences, export) — requires a valid access token, and a request to a
library-scoped route is further checked against the `X-Library-Id` header
against the token's own library claims.

The contract records this per-operation: an operation with `security: []`
is one of the allowlisted public routes above; every other operation
inherits the document-level `bearerAuth` requirement. Both are kept honest
by an automated check — see Keeping this contract honest, below.

## Error shape

Every error response shares one shape, regardless of endpoint:

```json
{
  "error": {
    "code": "NotFound",
    "message": "work not found in your library",
    "correlationId": "00000000-0000-0000-0000-000000000000"
  }
}
```

`code` is machine-readable, `message` is written for a person and never
contains a stack trace, a file path, or a SQL fragment, and
`correlationId` matches the ID in the server's own structured logs — the
thing to hand back when reporting a bug.

## Keeping this contract honest

Two independent checks run in CI (`internal/testutil/contracttest`) on
every change:

- **Response-shape validation**: every response a real handler returns is
  checked against the spec's schema for that operation. A handler that
  drifts from its documented response shape fails the build.
- **Route completeness**: every path under `/api/v1/` that has a
  registered handler must appear in the spec, and vice versa.
- **Security-annotation agreement**: every operation's `security`
  requirement is checked against `AuthMiddleware`'s own public-path list,
  independent of any HTTP round trip. This exists because the two other
  checks above validate response *shape*, not whether an operation is
  correctly documented as requiring authentication — a gap that let 33
  operations drift to an incorrect `security: []` annotation before this
  check was added for the v1.0.0 release. It closes that specific gap;
  see `TestSecurityAnnotationsMatchAuthMiddleware`.

If you add or change an endpoint, update `api/openapi.yaml` in the same
change. CI will fail the build if the spec and the implementation
disagree.
