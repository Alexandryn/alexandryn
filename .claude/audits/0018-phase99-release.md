# Security audit: Phase 99 — Release (packaging, container image, release CI)

| | |
|---|---|
| **Scope** | Everything Step 4/5's own work introduced or touched: the Electron packaging pipeline and its security fuses, the Docker image and compose stack, `.github/workflows/release.yml`, and the OpenAPI contract's authentication annotations (fixed earlier in this phase, re-verified here as part of the release surface). Does not re-audit application logic already covered by audits 0012–0017. |
| **Auditor** | Claude (Sonnet 5), self-review |
| **Date** | 2026-09-17 |
| **Commit** | `9d8488e` |
| **Verdict** | Clear — three findings, all fixed during this audit, none open |

## Scope and method

Code reading plus direct manual probing of the actual built artifacts, not
inference from config: built the Linux Electron package and read the fuse
bits back off the real packaged binary; built and ran the Docker image
under `docker compose --profile bundled-db`, inspected its layers
(`docker history`), its filesystem as the running non-root user, and its
actual environment-variable resolution (`docker compose config`); traced
the JWT/MFA-ticket signing key derivation in source; read
`.github/workflows/release.yml`'s secret handling directly.

**Authorization-control tracing**: not applicable to this audit's own new
surface — this phase's work is packaging and CI, not a new
tenant-scoped data path. The reading/library authorization surface this
directive governs was last re-traced in audit 0016 and isn't touched
here. Where this audit does discuss authentication (token type, key
separation), the claims below are checked against the actual signer code
in `internal/auth/jwt.go`, not against a spec description of it.

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| Docker image build context → published layers | Anyone who can pull or inspect the image | No secret, credential, or `.env` value survives into a layer |
| `docker-compose.yml`'s tracked default profile | A user who runs `docker compose up` without reading anything first | The default is safe to run as-is: no exposed port, no weak credential silently accepted |
| Electron packaged binary | Anyone who obtains the installer or extracts the ASAR | Security fuses (`RunAsNode` off, etc.) are actually set on the artifact a user runs, not just requested in config |
| JWT/MFA-ticket signing | A holder of a signature-valid token minted for one purpose | Cannot present it as a different-purpose token even if the type-assertion check were somehow bypassed |
| `release.yml`'s GHCR push | GitHub Actions log viewers | No credential value appears in a log line |

## Adversarial questions asked

- **Packaging attacker** — can a crafted installer or container image bypass Electron fuses, leak secrets from image layers, or introduce a supply-chain backdoor?
  - Fuses: built the real Linux target and read the fuse wire back off the packaged binary (`getCurrentFuseWire`, not just `flipFuses`'s return value) — `RunAsNode`/`EnableNodeCliInspectArguments`/`EnableNodeOptionsEnvironmentVariable` disabled, `EnableCookieEncryption`/`OnlyLoadAppFromAsar` enabled, matching the requested posture exactly. `electron/scripts/afterPack.cjs` fails the build itself if the readback disagrees with the request — this isn't a config that could silently drift from the artifact.
  - Image layers: `docker history --no-trunc` on the built image shows exactly five layers — the alpine base, a user-creation step, the binary `COPY`, `USER`, `HEALTHCHECK`, `ENTRYPOINT`. No secret material, no build-stage layer (multi-stage build discards it), no `.env` file. `.dockerignore` independently excludes `.env`, `.env.*`, `*.pem`, `*.key`, `*.p12`, `*.keystore`, and `.git` from the build context in the first place.
  - Supply-chain: `electron-builder` and `@electron/fuses` are both first-party/de-facto-standard tooling already justified in this phase's own commit (§9). No new install script runs without the existing `allowScripts` allowlist gate (`electron-winstaller`'s was added and reviewed, not silently permitted).
- **Network attacker** — does the released Docker image expose the library without authentication in any default configuration? Does the TLS condition hold?
  - `docker compose config` on the tracked file publishes no port for `backend` (guard-enforced by `scripts/check-compose-published-port.sh`, which scans the file on disk independent of trust). `internal/config/bindaddress.go` independently refuses to start if `BIND_ADDRESS` resolves to a publicly-routable class without a certificate — verified by trying the naive fix (`0.0.0.0`) during this phase's own Docker work and watching the server correctly refuse to start. The only way to reach the container from the host at all is `docker-compose.override.yml.example`, git-ignored, documented as loopback-only until the ADR 0017 TLS conditions are met.
- **Authentication attacker** — does the released build correctly enforce token type? Does packaged config create an unauthenticated path?
  - `AuthMiddleware` wraps the entire route mux (`cmd/server/main.go`'s outermost-in chain comment: `... -> auth -> routing`); the only exemptions are the explicit `IsPublicPath` allowlist (setup/login/refresh/logout/password-reset/TOTP-verify/pairing-verify, plus `/healthz`, `/readyz`).
  - Token-type separation is not claim-checking alone: `NewJWTSigner` derives a **second**, independent HKDF subkey (`mfa-ticket-signing-v1`) from the already-derived `jwt-signing-secret-v1` subkey specifically for MFA tickets (`internal/auth/jwt.go:79-91`) — a signature-valid MFA ticket cannot verify against the access-token secret even if the `typ` claim check were bypassed. `VerifyAccessToken` additionally asserts `claims.Type == TokenTypeAccess`. Both layers present, traced in source, not assumed from the audit-0012 fix description.
  - `CORS_ALLOWED_ORIGINS` has no default (`categoryOptionalNoDefault`) — unset means no cross-origin requests are permitted, not a permissive default.
- **Data attacker** — does the installer or container image leak credentials, session tokens, home-directory paths, or reading content in logs or the packaged binary?
  - Found and fixed during this audit: see A-18-01 below.
  - `release.yml`'s GHCR login passes the token and actor through `env:` and references them as shell variables (`$GHCR_TOKEN`, `$ACTOR`), not interpolated directly into the `run:` command — the value never appears as literal text in the workflow file or, consequently, in a log echo of the command itself.

## Findings

| ID | Severity | Title | Status |
|---|---|---|---|
| A-18-01 | Medium | Credential-encryption key had no persistent volume in the container target | Fixed (`9d8488e`) |
| A-18-02 | Low | Bundled Postgres password defaulted to a well-known literal | Fixed (`cae8cc1`) |
| A-18-03 | High (documentation-integrity, not a live vulnerability) | OpenAPI contract mis-documented 33 authenticated endpoints as public | Fixed (`b4037d2`, earlier in this phase) |

### A-18-01 — Credential-encryption key had no persistent volume in the container target

**Severity:** Medium

**Component:** `docker-compose.yml`, `internal/adapters/crypto/keyfile.go`

**Description** — `cmd/server/run.go` resolves the app-data directory
(where the source-credential AES-256 master key and the ACME certificate
cache live) from `os.UserConfigDir()`, which for this image's non-root
`app` user resolves to `/home/app/.config/alexandryn` — verified
empirically inside a running container (`$HOME=/home/app`,
`os.UserConfigDir()` → `$HOME/.config`). `docker-compose.yml` declared a
named volume for Postgres data but not for this directory, which lives
in the container's own writable layer.

**Impact** — Every container recreation (an image rebuild, an upgrade,
a plain `down` followed by `up` against a replaced container) discarded
this directory. `keyfile.go` handles a missing key file without
crashing — it warns and generates a replacement — but every existing
encrypted-at-rest source credential becomes permanently undecryptable
the moment that happens (`ErrDecrypt` forever; the ciphertext was sealed
under a key that no longer exists), and every existing user session's
signing key changes at the same time. This is exactly the failure mode
`keyfile.go`'s own comment describes as the *degraded recovery path*,
happening by default on routine operations rather than only in a genuine
key-loss incident.

**Preconditions** — Running the Docker deployment target, having stored
at least one source credential, and recreating the `backend` container
(common: any image update).

**Reproduction** — `docker compose --profile bundled-db up --build
--wait`; note the key file's hash inside the container; `docker compose
down` (container removed, not just stopped); `up` again; the key file
is regenerated with a new value (verified before the fix — the hash
changed; after the fix, verified identical across a full removal and
recreation).

**Recommendation** — Add a named volume covering
`/home/app/.config/alexandryn`, matching the existing `postgres-data`
pattern.

**Resolution** — `docker-compose.yml` now declares an `app-data` volume
mounted at that path. Verified: the mount point is writable by the
non-root `app` user with no permission mismatch (0600 key file, 32
bytes, owned by `app:app`), and the key's hash is identical before and
after a full container removal and recreation. Commit `9d8488e`.

---

### A-18-02 — Bundled Postgres password defaulted to a well-known literal

**Severity:** Low

**Component:** `docker-compose.yml`, `.env.example`

**Description** — The `bundled-db` profile's `POSTGRES_PASSWORD`
defaulted to the literal `admin` whenever `.env` didn't set it. The
`.env.example` line showing this default was commented out, so a
first-time self-hoster who didn't read it closely could bring up the
bundled database with that credential without realizing they'd made a
choice.

**Impact** — Low on its own: the bundled Postgres publishes no port by
default (network-isolated to the compose bridge network, reachable only
by the `backend` container or something with host/container shell
access), so this isn't remotely exploitable in the shipped default
configuration. It's exactly the kind of default the constitution's "the
user configured it, so it's fine" prohibition exists for, though: it
degrades the moment a user adds their own port mapping for debugging, a
sidecar reverse proxy, or any other legitimate reason to reach Postgres
directly, at which point `admin`/`admin` is a dictionary-first
credential.

**Preconditions** — Running the `bundled-db` profile without setting
`POSTGRES_PASSWORD`, plus some additional network exposure of the
Postgres port the user adds themselves.

**Reproduction** — `docker compose --profile bundled-db config` (before
the fix) resolved `POSTGRES_PASSWORD` to `admin` with no warning.

**Recommendation** — Require the value explicitly; fail closed rather
than default.

**Resolution** — `docker-compose.yml` now uses compose's
`${POSTGRES_PASSWORD:?message}` required-variable syntax — `docker
compose config` and `up` both refuse to proceed without it, with a
message pointing at `.env.example`. Verified: `config` fails clearly
with it unset; the full `up --build --wait` → persistence → `down`/`up`
cycle still succeeds with it set. Commit `cae8cc1`.

---

### A-18-03 — OpenAPI contract mis-documented 33 authenticated endpoints as public

**Severity:** High as a documentation-integrity defect; **not** a live
vulnerability — the running server enforces authentication correctly on
every one of these routes regardless of what the contract said.

**Component:** `api/openapi.yaml`

**Description** — 33 operations (the entire library/collections/
discover/sources surface plus the whole reading API — progress,
bookmarks, highlights, preferences, export) carried an explicit
`security: []` override, documenting them as unauthenticated, while
`AuthMiddleware` wraps the entire route mux and genuinely requires a
bearer token for all of them. The existing contract test
(`internal/testutil/contracttest`) never caught this because it
deliberately bypasses authentication (`NoopAuthenticationFunc`) to
isolate response-shape validation from auth enforcement — a real,
acknowledged gap in what that test guarantees, not a defect in its
design.

**Impact** — Anyone relying on the published contract — an external
integrator, a future contributor, a security reviewer scoping this
release — would have concluded the majority of this API requires no
authentication at all. That's the same *shape* of mistake audit
0012-C1 found at the code layer, recurring one layer up, in the
document a release ships as the source of truth for its own API
surface.

**Preconditions** — None; this was a static documentation defect,
discoverable by reading the file.

**Reproduction** — `grep -c 'security: \[\]' api/openapi.yaml` before
the fix: 44 (11 legitimately public — health checks and the
unauthenticated auth/pairing flows — plus these 33).

**Recommendation** — Remove the incorrect overrides; add a test that
compares every operation's declared security requirement against
`AuthMiddleware.IsPublicPath` directly, independent of any HTTP round
trip, so this can't silently recur.

**Resolution** — Fixed earlier in this phase, before this audit began
(Step 3): the 33 overrides removed, `TestSecurityAnnotationsMatchAuthMiddleware`
added and proven to catch the regression (run red against the unfixed
spec — 33 failures — before the fix, green after). Re-verified as part
of this audit by re-running that test and by re-reading the current
`api/openapi.yaml`. Commit `b4037d2`.

## What was not examined

- **macOS and Windows installer artifacts** — this environment is
  Linux-only; the fuse-verification and packaging logic were proven on
  the Linux target and read from source for the other two (the
  `afterPack.cjs` hook and its platform-specific binary-path resolution
  are platform-conditional but identical code, not a separate
  implementation per OS), but the actual `.dmg`/`.exe` artifacts
  themselves are unverified pending CI, which is currently blocked
  (GitHub Actions org billing issue, unrelated to this repository's
  code — see the phase's own status tracking).
- **Code signing** — deliberately unsigned; already recorded as a known
  release gap in `electron-builder.yml`'s own comments and this phase's
  earlier status reports, not re-litigated here.
- **The actual GHCR push** — the workflow step was read and reasoned
  about, not executed (no registry credentials available in this
  session, correctly so — `GITHUB_TOKEN` only exists inside the Actions
  runner). CI blocked, so this is genuinely unverified end-to-end.
- **`docs` and `website` repos** — don't exist yet (explicitly deferred
  by the maintainer to later in this phase); nothing to audit.
- **Application-layer authorization surface** (RBAC, tenant scoping,
  the reading API's user/library predicates) — last re-traced in audit
  0016; this phase didn't touch that code, so it wasn't re-walked here.
