# Security audit: Phase 16 — Whole-application security hardening

| | |
|---|---|
| **Scope** | The entire application at branch tip `feat/phase16-security-hardening` (`origin/main` = `1c60608`): `cmd/`, `internal/`, `web/src/`, `electron/src/`, `scripts/`, `.github/workflows/`, `api/`, `docker-compose.yml`, `Dockerfile`, `supabase/`, dependency manifests (`go.mod`, `go.sum`, `package.json`, `package-lock.json`). |
| **Auditor** | Claude Sonnet 5 (Claude Code) — specialist fan-out: `agent-skills:security-auditor`, `agent-skills:code-reviewer`, `agent-skills:test-engineer`, `agent-skills:web-performance-auditor`; plus `security-and-hardening` and `ci-cd-and-automation` skills inline; plus a separate maintainer-run `/code-review ultra` cross-check. |
| **Threat model** | Four-Attacker (Constitution §10) + STRIDE, applied to the application as a whole across the three phase-01 trust boundaries: Renderer/Main, Host/LAN, System/Source. |
| **Date** | 2026-09-07 |
| **Commit** | `1c60608` |
| **Verdict** | **HIGH remediation complete (2026-09-08)** — 120 findings filed as GitHub issues (#86–#304, deduplicated). 0 Critical. All **16 High** resolved RED → GREEN, one commit per issue (see the Remediation section below and `tasks/todo-phase16-security.md`). 95 Medium/Low/Informational triaged: a handful fixed opportunistically (#106, #119, #198), the rest scheduled to Phase 17 / post-release, each recorded against its issue. CI/CD hardened (ADR 0033, ADR 0034). Awaiting the maintainer phase-close gate. |

---

## Remediation (2026-09-08)

The maintainer directed the 16 actionable High findings and the named
CI/CD gaps to be worked directly. Each fix is its own commit on
`feat/phase16-security-hardening`, failing-first test then implementation,
with an integration test and (for tenant-scoped endpoints) a cross-tenant
negative test where the finding warranted one.

| Finding | Fix | Verification |
|---|---|---|
| #86 SSRF | `sources.GuardedTransport` — dialer `Control` hook rejects the resolved peer IP after DNS resolution (rebinding defence); link-local/CGNAT/multicast always blocked, loopback/RFC1918 blocked unless `SOURCE_ALLOW_PRIVATE_ADDRESSES` | `IsBlockedDialIP` table; transport refuses loopback; OPDS `Probe`+`Resolve` to a loopback source rejected by default |
| #88 catalog scoping | `domain.LibraryQuery.LibraryID` + `FindWorkDetail(…, libraryID)`; `work_repository.go` refactored to one shared CTE with `le.library_id = $3` / `c.library_id = $N` on every join; handlers resolve + re-check membership | `TestWorkRepository_LibraryScoping` — two-library isolation across list + detail incl. collection membership |
| #89 MFA brute-force | per-IP limiter (parity with login) + per-user `MFAUserLimiter` (5 burst, 1/min) checked after the ticket resolves the user | `TestTOTPVerify_RateLimited` — per-IP trips; per-user trips across rotating source addresses |
| #90 sync cursor | push endpoint advances the device pull cursor only when `seq == cursor+1`; the returned `cursor` is then safe to persist | `TestSyncProgressHandler_PushCursorNotUsableAsPullCursor` (fails with cursor=7 before the fix) |
| #262 library authz | `callerIsLibraryMember` / `callerIsLibraryAdmin`; `GetLibraryHandler` 404s non-members, `ListMembersHandler` + `CreateInvitationHandler` are library-admin only | outsider gets 404/403/403; a member with the admin role (not a global admin) can list members |
| #294 reaper | `observability.NewReaper(...).Start(ctx)` wired into `run.go` next to the event store | `TestReaper_Start_PurgesOnTick` — the ticker loop deletes an expired row |
| #296 nested redaction | `sanitizeMap`/`sanitizeValue` recurse into maps and slices; `note` added to `prohibitedPayloadKeys` | `TestNewSystemEvent_SanitizesNestedPayload` + redaction-proof integration test extended (nested `detail`, `note`, token-in-slice) |
| #91 web 401 | `QueryCache`/`MutationCache` `onError` → one shared `refreshSession()` → refetch or `clearSession()` + `/login?next=`; 4xx never retries (#227) | `queryClient.test.ts` — 404 not retried; 401 → one refresh, token cleared, redirect |
| #92 capability spinner | `CapabilityState` gains an `error` variant with `retry()`; fail-closed preserved (never `granted` on failure) | `capability.test.tsx` — failed bootstrap shows an alert + retry, no host-only content |
| #93 placeholder routes | real `SettingsIndex` + `MoreScreen` on a shared `NavList` | `SettingsIndex.test.tsx` — both link to their real targets |
| #94 auth tokens | auth screens + Titlebar + Libraries + DevicesSettings ported to the real `@theme` token set | `check:token-styling` + `tokens:check-contrast` clean; build green |
| #95 invite token | `AcceptInviteScreen` Sign In carries `state.from`; `LoginScreen` honours `from` → `?next=` → `/library` | folded into `queryClient.test.ts` / manual flow |
| #97 MFA modal a11y | both MFA modals rebuilt on the shared Radix `Modal`; `inputMode="numeric"` + `autoComplete="one-time-code"`, focus on the code field | `MfaPromptModal.test.tsx` — dialog role + name, focus, OTP hints, Escape |
| #99 export bypass | `http.ts` `getBlob()` (auth headers) + `useReadingExport` mutation; Reader shows pending / inline error | `reading.test.tsx` — export request carries `Authorization` + `X-Library-Id` |
| #100 code splitting | `React.lazy` per route (`lazyScreens.ts`) behind `Suspense`; initial chunk 597→442 KB (179→135 KB gzip) | `npm run build` chunk report; route table test green through Suspense |
| #102 feed polling | `activityPollInterval` — off when hidden, 5s active, 60s idle floor | `activity.pollInterval.test.ts` |

CI/CD: #121 (SHA-pin), #123 (`permissions`), #125 (`gosec`), #126 (npm
Dependabot), #128 (CODEOWNERS), #133 (guard widened), #198 (artifact
version align), #205 (artifact checksum) — ADR 0033 (coverage floor),
ADR 0034 (supply-chain posture).

Opportunistic Medium/Low: #106 (MFA-ticket HKDF subkey), #119
(`import.go`/`device_sync.go` → `writeDomainError`).

Everything else is triaged in `tasks/todo-phase16-security.md` with a
Phase 17 / post-release destination per issue.

## Scope and method

This is a consolidation audit (Phase 16, roadmap). Method:

1. **Code reading**, every wired path across the three trust boundaries. Not a
   sampling — the preload surface is enumerated, every HTTP handler is listed and
   traced, every source-facing parser is read.
2. **Mandatory authorization trace.** For every endpoint that reads or writes
   data scoped to a user, a library, a device, or any other tenant boundary:
   the *actual wired* handler → repository → SQL is traced and the tenant/user
   predicate is quoted from the query text (or the middleware check). A scoped
   repository method existing, a migration column, or spec text is **not**
   accepted as evidence. This is the control that audit 0012 got wrong (review
   0050): per-user reading scoping and token-type assertion were certified from
   the repository and migration layers without tracing one request to its SQL.
3. **Dependency review** — `govulncheck`, `npm audit`, and a license pass over
   direct and transitive dependencies, cross-checked against §9's recorded
   reasons.
4. **CI/CD review** — the GitHub Actions workflow's supply-chain surface (action
   pinning, `GITHUB_TOKEN` permissions, cross-job artifact trust, secret
   exposure, `pull_request_target`), and the `scripts/check-*` guard scripts.
5. **Automated-testing review** — coverage on security-relevant paths, presence
   of cross-tenant/negative tests, skipped or flaky tests, the unset coverage
   threshold.
6. **Manual probing** where a static read is inconclusive.

Every confirmed finding is filed as its own GitHub issue (label scheme in the
phase README, G0-3) and recorded in the Findings table below with a link. The
sweep does not triage findings away at capture time (G0-1).

## Trust boundaries examined

| Boundary | Untrusted side | Assumption being made |
|---|---|---|
| Renderer → Main (IPC) | The renderer process and anything it loads | The preload exposes an enumerated operation list; every argument is validated in the main process |
| Main → OS | Book files, paths, spawned processes | Paths from sources are attacker-chosen strings; the pg supervisor is spawned with a fixed argv |
| LAN client → Host (HTTP API) | Any device on the network, and a hostile page in the local user's browser | Loopback-default bind; broader bind requires auth + ADR 0017 TLS; every data handler is user-and-library scoped |
| Authenticated user → API | A credentialed user wanting more than their share | Every tenant-scoped query carries a `user_id` / `library_id` predicate; token type is asserted, not just signature |
| Source → System | A user-configured source: its metadata, filenames, redirects, response bytes | Size limit, shape check, timeout on every response; filenames sanitised; redirects bounded |
| Book file → Parser | A file that is legal to possess but hostile to parse | Zip-bomb / path-traversal / malformed-container defence in the EPUB/format path |
| CI job → CI job | Artifacts and state passed between GitHub Actions jobs | `web/dist` and the server binary handed between jobs are trusted; the workflow token is minimally scoped |
| Dependency tree | Every direct and transitive package | No known CVE at or above `high`; each direct dependency has a recorded reason (§9) |

## Adversarial questions asked

Worked explicitly per Constitution §10, against the whole application:

- **Malicious user of this instance** — what cross-tenant read/write/delete is
  reachable? Token confusion between access, MFA, and pairing grants? Role
  escalation through an unguarded handler?
- **Malicious source** — what do its metadata, filenames, redirects, and
  response bytes achieve? Oversized/slow/never-arriving responses? SSRF via a
  source URL or redirect?
- **Unauthenticated LAN device** — what is reachable without a credential? What
  does a hostile page in the local user's browser reach via CSRF / DNS
  rebinding? Do the security headers and CSP hold?
- **Malformed / empty / enormous / duplicated / slow / absent input** — on every
  external surface: IPC, HTTP body, source response, book file, sync payload.
- **What is logged that shouldn't be** — credentials, tokens, home paths, book
  content, reading position — re-verified against every log call, including
  those added since the phase 15 redaction test.
- **What happens if two run at once** — concurrent sync, concurrent import,
  concurrent job cancellation, reaper vs. writer.

## Findings

120 findings filed as individual GitHub issues (#86–#304, labels `severity:*`,
`area:*`, `phase-16`), deduplicated to one issue per finding. Each issue carries
the finding, location, impact, reproduction, and recommended fix. Severity is
rated for impact **in this system**, not the textbook worst case (Constitution
§10); for non-security findings the `severity:` label is a priority proxy.

The table below is the index for the sweep (#86–#292). The `/code-review ultra`
additions (#293–#304) and the three verification corrections are in the two
sections after the reconciliation notes. The `#` column is the sweep's running
number; two cells (#87, #262) carry their corrected severity with a note.

**Counts (after the ultra pass and the verification corrections):** 0 Critical ·
16 High · 48 Medium · 47 Low · 9 Informational — 120 total (#86–#304).
**By area:** web 47 · backend 47 · ci 13 · tests 9 · deps 4 · electron 2 · docs 1.

Six findings (#260, #262, #264, #265, #267, and the guard-script coverage gap)
record surfaces the sweep did **not** fully reach — they are filed as findings
per the phase strategy ("boundaries with no negative test are themselves a
finding") and must be closed by completing the review, not by assertion.

| # | Severity | Area | Title | Issue | Origin |
|---|---|---|---|---|---|
| 1 | High | backend | SSRF: source base URL / OPDS client has no private-IP guard or DNS-rebinding defense | [#86](https://github.com/Alexandryn/alexandryn/issues/86) | backend BE-01, security SEC-07, tests TEST-09 |
| 2 | Medium | backend | Collections API has no library/tenant scoping (latent — write path does not set library_id) | [#87](https://github.com/Alexandryn/alexandryn/issues/87) | security SEC-01, tests TEST-02; downgraded on verification |
| 3 | High | backend | GET /api/v1/library and GET /api/v1/works/{id} are not library-scoped (cross-library holdings disclosure) | [#88](https://github.com/Alexandryn/alexandryn/issues/88) | security SEC-03, tests TEST-01 |
| 4 | High | backend | TOTP MFA verification endpoint has no rate limiting (MFA brute-force / bypass) | [#89](https://github.com/Alexandryn/alexandryn/issues/89) | security SEC-02 |
| 5 | High | backend | Device sync push cursor is unsafe as a pull cursor (silent cross-device data loss) | [#90](https://github.com/Alexandryn/alexandryn/issues/90) | backend BE-02, tests TEST-08; prior-session memory note re PR #82 watermark concern |
| 6 | High | web | No global auth-failure handling; token expiry puts the whole app in an unrecoverable dead state | [#91](https://github.com/Alexandryn/alexandryn/issues/91) | frontend FE-01; compounded by FE-21 (retry on client errors) |
| 7 | High | web | CapabilityProvider renders an infinite spinner on any bootstrap fetch failure | [#92](https://github.com/Alexandryn/alexandryn/issues/92) | frontend FE-02 |
| 8 | High | web | /settings and /more routes are bare placeholders; primary nav dead-ends, mobile loses half the app | [#93](https://github.com/Alexandryn/alexandryn/issues/93) | frontend FE-03 |
| 9 | High | web | Auth/entry screens built against an undefined design-token vocabulary; styling and contrast broken | [#94](https://github.com/Alexandryn/alexandryn/issues/94) | frontend FE-04 |
| 10 | High | web | Invitation-acceptance flow loses the token when the user is not signed in | [#95](https://github.com/Alexandryn/alexandryn/issues/95) | frontend FE-05; related FE-08 (LoginScreen return dest) |
| 11 | High | web | MFA modals are hand-rolled divs: no dialog role, focus trap, Escape, or focus return | [#97](https://github.com/Alexandryn/alexandryn/issues/97) | frontend FE-06 |
| 12 | High | web | downloadReadingExport bypasses the HTTP layer: no auth header, silent failure | [#99](https://github.com/Alexandryn/alexandryn/issues/99) | frontend FE-07 |
| 13 | High | web | Entire app ships as one JS chunk; no route-level code splitting | [#100](https://github.com/Alexandryn/alexandryn/issues/100) | web-performance PERF-01 |
| 14 | High | web | Activity feed polls every 10s forever from the always-mounted sidebar badge | [#102](https://github.com/Alexandryn/alexandryn/issues/102) | web-performance PERF-04 |
| 15 | Medium | backend | Import candidate endpoints are not library-scoped (cross-library enumerate / sabotage / confirm) | [#104](https://github.com/Alexandryn/alexandryn/issues/104) | backend BE-03, tests TEST-05 |
| 16 | Medium | backend | MFA ticket is signed with the access-token HKDF subkey, not its own | [#106](https://github.com/Alexandryn/alexandryn/issues/106) | backend BE-09, security SEC-13 |
| 17 | Medium | backend | MFA can be re-enrolled or silently disabled with only an access token (no password / step-up) | [#107](https://github.com/Alexandryn/alexandryn/issues/107) | security SEC-04 |
| 18 | Medium | backend | Sync progress path cannot carry a backward progress correction (epoch clamp) | [#109](https://github.com/Alexandryn/alexandryn/issues/109) | backend BE-04 |
| 19 | Medium | backend | Sync progress accepts an unvalidated precise-position edition id | [#111](https://github.com/Alexandryn/alexandryn/issues/111) | backend BE-11 |
| 20 | Medium | backend | GET /api/v1/library/finished runs an unbounded, unpaginated query | [#112](https://github.com/Alexandryn/alexandryn/issues/112) | backend BE-05 |
| 21 | Medium | backend | GET /api/v1/sync/reading delta queries have no LIMIT and 'since' defaults to 0 | [#114](https://github.com/Alexandryn/alexandryn/issues/114) | backend BE-07 |
| 22 | Medium | backend | No usable index for the leaderboard / finished-works hot path | [#116](https://github.com/Alexandryn/alexandryn/issues/116) | backend BE-06 |
| 23 | Medium | backend | COALESCE(col,'') = COALESCE($,'') scoping predicate is index-defeating and NULL-owner-matching | [#118](https://github.com/Alexandryn/alexandryn/issues/118) | backend BE-08 |
| 24 | Medium | backend | Raw err.Error() text is written into HTTP response bodies | [#119](https://github.com/Alexandryn/alexandryn/issues/119) | backend BE-10 |
| 25 | **High** | backend | GetLibraryHandler / ListMembersHandler have no authz — member email disclosure to any reader (trace completed) | [#262](https://github.com/Alexandryn/alexandryn/issues/262) | security SEC 'what could not reach', tests TEST-04; upgraded on verification |
| 26 | Medium | backend | cmd/pg-supervisor / Postgres spawn surface not audited | [#264](https://github.com/Alexandryn/alexandryn/issues/264) | security SEC 'what could not reach', tests TEST-12 |
| 27 | Medium | ci | GitHub Actions are pinned to mutable major tags, not commit SHAs | [#121](https://github.com/Alexandryn/alexandryn/issues/121) | ci CI-01, security SEC-06 |
| 28 | Medium | ci | CI workflow has no permissions: block (default GITHUB_TOKEN scope) | [#123](https://github.com/Alexandryn/alexandryn/issues/123) | ci CI-02, security SEC-06 |
| 29 | Medium | ci | No SAST / security linter in CI | [#125](https://github.com/Alexandryn/alexandryn/issues/125) | ci CI-03 |
| 30 | Medium | ci | package-lock.json is not in CODEOWNERS; pnpm-lock.yaml (which does not exist) is listed instead | [#128](https://github.com/Alexandryn/alexandryn/issues/128) | ci CI-05 |
| 31 | Medium | deps | Dependabot does not watch npm (web/, electron/, the root lockfile) | [#126](https://github.com/Alexandryn/alexandryn/issues/126) | ci CI-04, security SEC-06 |
| 32 | Medium | docs | No LICENSE file in the repository | [#130](https://github.com/Alexandryn/alexandryn/issues/130) | deps DEP-02 |
| 33 | Medium | electron | Electron fuses configuration not found — **re-scoped to Phase 99** (no packaging pipeline yet) | [#260](https://github.com/Alexandryn/alexandryn/issues/260) | security SEC 'what could not reach'; removed from Phase 16 close gate |
| 34 | Medium | tests | No coverage tool or threshold in CI (open since phase 03); frontend runs no coverage at all | [#131](https://github.com/Alexandryn/alexandryn/issues/131) | tests TEST-07, security SEC-12 |
| 35 | Medium | tests | check-user-scoped-reading.sh has a narrow glob, is trivially bypassed, and its SQL branch has no self-test | [#133](https://github.com/Alexandryn/alexandryn/issues/133) | tests TEST-06, security SEC-12 |
| 36 | Medium | tests | No cross-tenant / negative test for collections, library, works, or import endpoints | [#135](https://github.com/Alexandryn/alexandryn/issues/135) | tests TEST-01/02/03/05 (meta), security SEC-12 |
| 37 | Medium | tests | No concurrency test on the phase-14 sync surface | [#137](https://github.com/Alexandryn/alexandryn/issues/137) | tests TEST-08, backend BE-02 |
| 38 | Medium | web | Switching the active library leaves stale cross-library data cached; one invalidation key is wrong | [#138](https://github.com/Alexandryn/alexandryn/issues/138) | frontend FE-09 |
| 39 | Medium | web | DevicePairingModal: prefers-reduced-motion disables the functional expiry countdown and SR announcements | [#140](https://github.com/Alexandryn/alexandryn/issues/140) | frontend FE-10 |
| 40 | Medium | web | Fire-and-forget destructive mutations swallow failures (revoke device, delete source, revoke pairing) | [#142](https://github.com/Alexandryn/alexandryn/issues/142) | frontend FE-11 |
| 41 | Medium | web | NetworkSettings form never loads the actual current settings | [#143](https://github.com/Alexandryn/alexandryn/issues/143) | frontend FE-12 |
| 42 | Medium | web | Reader claims to restore reading position but only restores the chapter, never the scroll offset | [#145](https://github.com/Alexandryn/alexandryn/issues/145) | frontend FE-13 |
| 43 | Medium | web | Reader highlight creation always sends a zero-length range (start === end); the feature does not work | [#147](https://github.com/Alexandryn/alexandryn/issues/147) | frontend FE-14 |
| 44 | Medium | web | Interface copy: marketing voice, apologising, pervasive Title Case (Constitution §11) | [#149](https://github.com/Alexandryn/alexandryn/issues/149) | frontend FE-15 |
| 45 | Medium | web | RouteError boundary covers only shell children, not public routes, RequireAuth, or AppShell | [#150](https://github.com/Alexandryn/alexandryn/issues/150) | frontend FE-16 |
| 46 | Medium | web | Pagination and page navigation lose keyboard focus | [#152](https://github.com/Alexandryn/alexandryn/issues/152) | frontend FE-17 |
| 47 | Medium | web | AccessScreen decodes the JWT client-side to make authz/UI decisions inside a screen component | [#154](https://github.com/Alexandryn/alexandryn/issues/154) | frontend FE-18 |
| 48 | Medium | web | Status queries fail silently: whole sections vanish with no error or retry | [#156](https://github.com/Alexandryn/alexandryn/issues/156) | frontend FE-19 |
| 49 | Medium | web | Multi-step pairing/enrolment flow carried entirely in router state; breaks on reload | [#157](https://github.com/Alexandryn/alexandryn/issues/157) | frontend FE-20 |
| 50 | Medium | web | No Content-Security-Policy on the served web app HTML | [#159](https://github.com/Alexandryn/alexandryn/issues/159) | frontend FE-31; related to the style-src finding |
| 51 | Medium | web | LoginScreen ignores the post-login return destination and offers no password reset | [#161](https://github.com/Alexandryn/alexandryn/issues/161) | frontend FE-08; related FE-05 |
| 52 | Medium | web | qrcode (~52 KB raw) is eagerly bundled for a rare host-only modal | [#163](https://github.com/Alexandryn/alexandryn/issues/163) | web-performance PERF-02 |
| 53 | Medium | web | Radix UI primitives are duplicated across packages in the bundle | [#164](https://github.com/Alexandryn/alexandryn/issues/164) | web-performance PERF-03 |
| 54 | Medium | web | No request cancellation anywhere; http.ts never forwards AbortSignal | [#166](https://github.com/Alexandryn/alexandryn/issues/166) | web-performance PERF-05 |
| 55 | Medium | web | WorkGrid 'virtualization' only grows; it never windows or unmounts rows | [#168](https://github.com/Alexandryn/alexandryn/issues/168) | web-performance PERF-06 |
| 56 | Medium | web | Import screen polls queued candidates every 2s unconditionally | [#169](https://github.com/Alexandryn/alexandryn/issues/169) | web-performance PERF-08 |
| 57 | Medium | web | Import candidate covers are delivered as inline base64 in JSON, and the incoming data: prefix is trusted | [#171](https://github.com/Alexandryn/alexandryn/issues/171) | web-performance PERF-09, frontend FE-22 |
| 58 | Low | backend | VerifyAccessToken still accepts an empty typ claim | [#173](https://github.com/Alexandryn/alexandryn/issues/173) | backend BE-12 |
| 59 | Low | backend | Sync side-effect errors are silently discarded | [#175](https://github.com/Alexandryn/alexandryn/issues/175) | backend BE-13 |
| 60 | Low | backend | local.Provider.Resolve does not re-check the file extension | [#176](https://github.com/Alexandryn/alexandryn/issues/176) | backend BE-14 |
| 61 | Low | backend | Activity job control is global, keyed on the global user role only | [#178](https://github.com/Alexandryn/alexandryn/issues/178) | backend BE-15, tests TEST-14 |
| 62 | Low | backend | openlibrary.GetWork has no aggregate deadline over ~22 sequential calls | [#180](https://github.com/Alexandryn/alexandryn/issues/180) | backend BE-16 |
| 63 | Low | backend | FinishedWorksHandler slow-query timer measures nothing | [#182](https://github.com/Alexandryn/alexandryn/issues/182) | backend BE-17 |
| 64 | Low | backend | Highlight PATCH always writes, burning a sync sequence | [#183](https://github.com/Alexandryn/alexandryn/issues/183) | backend BE-18 |
| 65 | Low | backend | JSON responses are not error-checked after WriteHeader | [#185](https://github.com/Alexandryn/alexandryn/issues/185) | backend BE-21 |
| 66 | Low | backend | Login endpoint leaks account existence via response timing | [#187](https://github.com/Alexandryn/alexandryn/issues/187) | security SEC-05 |
| 67 | Low | backend | MFA ticket is replayable within its 5-minute TTL | [#189](https://github.com/Alexandryn/alexandryn/issues/189) | security SEC-09 |
| 68 | Low | backend | App CSP still ships style-src 'unsafe-inline' (phase-16 TODO left in shipped code) | [#191](https://github.com/Alexandryn/alexandryn/issues/191) | security SEC-08 |
| 69 | Low | backend | Per-IP rate limiting degrades to global behind a reverse proxy; IPv6 /64 rotation is free | [#195](https://github.com/Alexandryn/alexandryn/issues/195) | security SEC-17 |
| 70 | Low | ci | upload-artifact@v7 paired with download-artifact@v8 | [#198](https://github.com/Alexandryn/alexandryn/issues/198) | ci CI-07 |
| 71 | Low | ci | No job-level timeout-minutes; no concurrency group | [#200](https://github.com/Alexandryn/alexandryn/issues/200) | ci CI-08 |
| 72 | Low | ci | No branch protection on main; 'never commit to main' is unenforced | [#202](https://github.com/Alexandryn/alexandryn/issues/202) | ci CI-06 |
| 73 | Low | ci | No automated license gate in CI | [#204](https://github.com/Alexandryn/alexandryn/issues/204) | deps DEP-03, tests TEST-11 |
| 74 | Low | ci | Cross-job artifact trust chain is implicit | [#205](https://github.com/Alexandryn/alexandryn/issues/205) | ci CI-09 |
| 75 | Low | ci | npm audit ignores moderate advisories | [#207](https://github.com/Alexandryn/alexandryn/issues/207) | tests TEST-11, security SEC-06 |
| 76 | Low | ci | Bundle-size CI check cannot see the monolith | [#225](https://github.com/Alexandryn/alexandryn/issues/225) | web-performance PERF-15 |
| 77 | Low | deps | golang.org/x/crypto v0.55.0 carries 3 advisories (not on any call path) | [#197](https://github.com/Alexandryn/alexandryn/issues/197) | deps DEP-01, security SEC-10 |
| 78 | Low | electron | Electron recovering-banner injects executeJavaScript via string interpolation | [#193](https://github.com/Alexandryn/alexandryn/issues/193) | security SEC-11, frontend FE-30 |
| 79 | Low | tests | Playwright has no forbidOnly; CI retries can mask flake | [#209](https://github.com/Alexandryn/alexandryn/issues/209) | tests TEST-10 |
| 80 | Low | tests | The real bundled-Postgres spawn and ACME issuance are skipped on every CI run | [#210](https://github.com/Alexandryn/alexandryn/issues/210) | tests TEST-12 |
| 81 | Low | tests | ~15 integration tests use real-clock time.Sleep for synchronization | [#212](https://github.com/Alexandryn/alexandryn/issues/212) | tests TEST-13 |
| 82 | Low | tests | api/openapi.yaml contract vs actual response shapes — over-disclosure check not performed | [#265](https://github.com/Alexandryn/alexandryn/issues/265) | tests (contract), security SEC 'what could not reach' |
| 83 | Low | tests | check-parameterized-queries.sh / check-import-boundaries.sh not audited for bypass | [#267](https://github.com/Alexandryn/alexandryn/issues/267) | tests, security SEC 'what could not reach' |
| 84 | Low | web | parseActivityEvents recomputed on every render (badge + screen) | [#214](https://github.com/Alexandryn/alexandryn/issues/214) | web-performance PERF-07 |
| 85 | Low | web | CapabilityProvider context value and can() closure recreated every render | [#216](https://github.com/Alexandryn/alexandryn/issues/216) | web-performance PERF-10 |
| 86 | Low | web | allWorks rebuilt with a fresh array identity every Library render | [#217](https://github.com/Alexandryn/alexandryn/issues/217) | web-performance PERF-11 |
| 87 | Low | web | Discover result covers are all loading=lazy; no priority hint for the first row | [#219](https://github.com/Alexandryn/alexandryn/issues/219) | web-performance PERF-12 |
| 88 | Low | web | No content-visibility on long scrolling regions | [#222](https://github.com/Alexandryn/alexandryn/issues/222) | web-performance PERF-13 |
| 89 | Low | web | Design fonts are referenced but never loaded or preloaded (latent CLS) | [#223](https://github.com/Alexandryn/alexandryn/issues/223) | web-performance PERF-14 |
| 90 | Low | web | retry: 2 default applies to auth and client errors | [#227](https://github.com/Alexandryn/alexandryn/issues/227) | frontend FE-21 |
| 91 | Low | web | Dismissed-import-failure IDs accumulate in localStorage unbounded | [#229](https://github.com/Alexandryn/alexandryn/issues/229) | frontend FE-23 |
| 92 | Low | web | deleteRequest throws on a 200 response with an empty body | [#230](https://github.com/Alexandryn/alexandryn/issues/230) | frontend FE-24 |
| 93 | Low | web | AppProviders creates a module-level singleton QueryClient, contradicting its own contract | [#232](https://github.com/Alexandryn/alexandryn/issues/232) | frontend FE-25 |
| 94 | Low | web | Reader iframe load handler registers scroll/selectionchange listeners with no cleanup | [#234](https://github.com/Alexandryn/alexandryn/issues/234) | frontend FE-26 |
| 95 | Low | web | Titlebar 'search' is a link styled as a text field | [#236](https://github.com/Alexandryn/alexandryn/issues/236) | frontend FE-27 |
| 96 | Low | web | Reader TOC entries pointing at non-linear or unmatched sections silently do nothing | [#237](https://github.com/Alexandryn/alexandryn/issues/237) | frontend FE-28 |
| 97 | Low | web | Redundant aria-label duplicating the visible label on search inputs | [#239](https://github.com/Alexandryn/alexandryn/issues/239) | frontend FE-29 |
| 98 | Low | web | Rejecting an import candidate is irreversible with no confirmation | [#241](https://github.com/Alexandryn/alexandryn/issues/241) | frontend FE-32 |
| 99 | Low | web | Post-navigation focus lands on a container with no announced heading on some routes/states | [#243](https://github.com/Alexandryn/alexandryn/issues/243) | frontend FE-33 |
| 100 | Low | web | SourceFormDialog validation errors are not tied to fields and don't move focus | [#245](https://github.com/Alexandryn/alexandryn/issues/245) | frontend FE-34 |
| 101 | Info | backend | Active library falls back to the default without a membership check | [#246](https://github.com/Alexandryn/alexandryn/issues/246) | backend BE-19 |
| 102 | Info | backend | GetSyncSequenceCeiling reads a non-transactional sequence value | [#248](https://github.com/Alexandryn/alexandryn/issues/248) | backend BE-20 |
| 103 | Info | backend | Enrolment-grant single-use check is TOCTOU | [#250](https://github.com/Alexandryn/alexandryn/issues/250) | security SEC-14 |
| 104 | Info | backend | Open Library client follows HTTP redirects unrestricted | [#251](https://github.com/Alexandryn/alexandryn/issues/251) | security SEC-15 |
| 105 | Info | backend | Reader HTML sanitizer allows data: URIs on <a href> | [#253](https://github.com/Alexandryn/alexandryn/issues/253) | security SEC-16 |
| 106 | Info | ci | Postgres service container uses a trivial password | [#259](https://github.com/Alexandryn/alexandryn/issues/259) | ci (inline) |
| 107 | Info | deps | npm deprecation warnings in the install tree | [#255](https://github.com/Alexandryn/alexandryn/issues/255) | deps DEP-04 |
| 108 | Info | deps | Three CC-BY-4.0 npm packages need attribution verification | [#257](https://github.com/Alexandryn/alexandryn/issues/257) | deps DEP-03 |

### Reconciliation notes

Where two reviewers rated the same finding differently, the filed severity is
the higher of the two with both readings recorded on the issue:

- **SSRF (#86):** security-auditor rated Low (admin-gated source creation);
  backend reviewer rated High (`Provider.Resolve` reachable by any user with an
  offering from that source, plus DNS rebinding). Filed **High**.
- **`GET /library` / `/works/{id}` scoping (#88):** security-auditor Medium
  (read-only); test-engineer High (horizontal IDOR on catalog data). Filed
  **High**.
- **MFA-ticket subkey (#106):** backend reviewer Medium (explicit reflex
  violation, review 0050 / audit 0012-C2); security-auditor Informational
  (defence-in-depth). Filed **Medium**.
- **Raw `err.Error()` in response bodies (#119):** the security-auditor's
  four-attacker pass concluded `(*domain.Error).Error()` returns only the
  curated message, so the domain-error path does not leak. The backend reviewer
  found specific call sites (`import.go`, `device_sync.go`) that bypass
  `writeDomainError` and pass a raw non-domain error. Both are correct; the
  finding stands for the bypassing call sites.

## `/code-review ultra` cross-check pass (2026-09-07)

Run against `main...HEAD` — a diff-scoped review, so its depth is on the
Phase 14 (device sync) and Phase 15 (observability) code, not the whole
application. 15 findings; 3 duplicated the sweep (sync cursor #90, leaderboard
slow-query timer #182, raw `err.Error()` #119 — cross-validation comments added
to each), 12 filed new as **#293–#304**. The reviewer reproduced the metrics
`r.Pattern` bug with a standalone Go test.

**The material outcome: four Phase 15 deliverables were certified but never
wired.** Phase 15 closed 2026-09-07 with audit `0015` "Clear" and all exit
criteria checked. Verified by grep and code trace during this pass:

| # | Phase 15 claim | Reality |
|---|---|---|
| #294 | Exit criterion "Activity-log retention reaper wired and tested" (task T1.4) | `observability.NewReaper` has zero callers in `cmd/`. The reaper is tested but never started. `system_events` is never purged — Constitution §8 / ADR 0031 retention is not enforced. |
| #296 | Audit `0015` "Clear"; redaction-proof CI test passes | `SanitizedPayload` strips only top-level keys; nested maps and the `note` key pass through. Reading content / position can reach `system_events` and the activity feed. The redaction-proof test evidently only exercises top-level keys. |
| #295 | Exit criterion `/api/v1/diagnostics` "returns correct metrics" | `SetPoolStatsProvider` / `SetQueueDepthProvider` are never called; diagnostics always reports zeroed pool stats and empty queue depth — a false "healthy" signal. |
| #302 | ADR 0030: metrics via `expvar` | The `expvar.NewMap("alexandryn_metrics")` is registered empty and never populated or served; `LatencyHistogram` bucket counts are dead; no overflow bucket above 10 s. |

These do not reopen Phase 15 on their own, but they are the same
"happy-path passed, definition of done not met" failure the constitution warns
about, and they are recorded here (as the Phase 14 PR #82 concerns were) so the
next close is done against the wired code. #293 (metrics cardinality leak via
empty `r.Pattern`), #298 (`RetryJob` re-runs completed jobs → duplicate
imports), #299 (`CancelJob` doesn't stop the worker), and #300 (Activity
"Pause" button actually cancels irreversibly) are correctness/UX findings in the
same code.

## Post-filing verification corrections

Spot-checking the filed issues against the code turned up three that were
mis-rated or mis-scoped at filing:

- **#87 (collections IDOR) — High → Medium.** `CreateCollectionHandler` →
  `INSERT INTO collections (id, name)` never writes `library_id`; every
  collection lands in the default library via the column default. Multi-library
  collections are not wired (the Phase 12 FR-4 retrofit covered reading data
  only). Real spec-conformance / defence-in-depth gap, but **latent** — there is
  no cross-library collection data to leak today. The same applies to the
  `sources` API (all `adminOnly`, no `library_id` predicate; per-library sources
  are "future" per `backend-library-namespaces.md` FR-5) — folded here rather
  than filed separately.
- **#262 (invitation/membership trace) — Medium → High, trace completed.**
  `GetLibraryHandler` and `ListMembersHandler` (`library_handlers.go:127,235`)
  perform **no authorization at all** and are wired with no `adminOnly` wrapper.
  `ListMembersHandler` returns username + email + role for **every member of any
  library** to any authenticated reader — spec FR-2 requires library admin. Live
  High: user/email enumeration, LAN-exposed post-Phase-13. `CreateInvitationHandler`
  and the library-mutation handlers check the *global* admin role, not the
  library-membership role (also an FR-1/FR-2 violation).
- **#260 (Electron fuses) — re-scoped to Phase 99, removed from the Phase 16
  close gate.** `electron/` has only build tooling — no `@electron/fuses`,
  electron-builder, or Forge config; there is no packaging pipeline to set fuses
  on, and packaging is Phase 99 (release). The Phase 16 roadmap outline scoped
  "CSP review", not fuses; adding "Electron fuse configuration reviewed" to the
  expanded exit criteria was scope creep and is corrected here.

Confirmed accurate and correctly attributed against the code:
#86, #88 (`LibraryHandler` never reads `ActiveLibraryFromContext`; multi-library
book placement *is* wired at `library_entry_repository.go:91`, so this one is
live), #89, #90, #94, #106, #119, #128, #130, #131, #149 (§11 quotes verbatim),
#191. The multi-library premise is legitimate — `backend-library-namespaces.md`
is `APPROVED` and explicitly requires these surfaces scoped; the `libraryInClaims`
middleware check (FR-3) is now shipped (`auth_middleware.go:144`).

## Severity guide

| | |
|---|---|
| **Critical** | Remote code execution, or unauthenticated access to the library from the network |
| **High** | Authentication bypass, arbitrary file read/write, renderer sandbox escape, credential leakage |
| **Medium** | Requires user interaction or an unusual configuration; limited data exposure; persistent DoS |
| **Low** | Narrow impact, high preconditions, or defence-in-depth that's missing |
| **Informational** | No impact today, but it makes a future mistake more likely |

Non-security findings (code quality, user-flow, workflow, test-coverage) are
filed with the same `severity:` labels used as a priority proxy and an
`area:` label, per G0-1/G0-3.

## Threat-model consolidation, per boundary

### Renderer → Main (Electron IPC)

**Held.** `contextIsolation: true`, `sandbox: true`, `nodeIntegration: false`.
The preload exposes exactly three argument-less operations, each Zod-validated
in the main process. External navigation is blocked and links are forced to the
system browser with scheme validation. Server config is written mode 0600 in a
0700 `mkdtemp` dir. One Low finding: the recovering-banner is injected via
`executeJavaScript` string interpolation (#193) — safe today, fragile pattern.
**Not verified:** the packaged binary's Electron fuses (#260) — no
`@electron/fuses` config was found; this is a phase-16 exit-criteria item.
`serverLifecycle.ts` / `serverProcess.ts` / `serverBinary.ts` binary-path
resolution and integrity were not read.

### LAN client → Host (HTTP API)

**Mostly held, with a cross-phase seam.** Loopback-default bind with a
fail-closed public-bind TLS gate checked against the actual address class, never
a flag. `AuthMiddleware` on every `/api/v1/` route with token-*type* assertion
and `X-Library-Id` validated against claims. Bearer (not cookie) auth makes the
API CSRF-safe by construction; the three unauthenticated state-changing POSTs
carry `OriginValidation`. Deny-by-default CORS with no `Allow-Credentials`.
Security headers on every response. The **reading / bookmarks / highlights /
preferences / sync / device / leaderboard** surfaces were traced
handler → repository → SQL and **are** genuinely `user_id` + `library_id`
scoped, with foreign/missing rows returning `NotFound` (no existence oracle),
and CI enforces it for that surface.

**The seam:** the phase-12 multi-library scoping was never extended to the
earlier **collections** (#87) and **library catalog / work detail** (#88) code,
and the **import candidate** endpoints (#104) authorise "may ingest" without
checking candidate ownership. `GetLibraryHandler` / `ListMembersHandler`
perform no authorization at all (folded into #88's cluster and #262). The CI
guard meant to prevent an IDOR recurrence (`check-user-scoped-reading.sh`) does
not scan any of these files (#133). This is audit 0012's miss (review 0050)
recurring on a wider surface, and it is the single most important outcome of
this sweep.

Auth-path findings: MFA verify has no rate limit (#89, High), MFA ticket shares
the access subkey (#106) and is replayable (#189), MFA can be re-enrolled with
only an access token (#107), the login timing oracle (#187), per-IP limiting
collapses behind a proxy (#195). The **invitation / membership** authorization
path was not fully traced (#262) and needs its own review.

### System → Source / book file

**Held.** Redirects disabled on the OPDS client, 5 MiB / 5 s caps, `SameOrigin`
on every response-derived URL. Local-folder traversal and symlink escape are
handled. Zip-bomb / entry-count / decompressed-size caps are enforced in the
extract path; EPUB/CBZ entries are read in memory with no `filepath.Join` /
`os.Create`, so no zip-slip. The gap is **SSRF** (#86, High): neither the OPDS
client nor the Open Library client (#251) screens the target IP for
private/loopback/link-local, and there is no DNS-rebinding defense. The
`local.Provider.Resolve` path does not re-check the file extension (#176). The
`cmd/pg-supervisor` spawn surface was not read (#264) and its E2E test is
skipped on every CI run (#210).

### CI job → CI job / dependency tree

`npm audit` and `govulncheck` (symbol level) are clean. Licenses are all
permissive (no GPL/AGPL/LGPL). The findings are hardening gaps, not active
vulnerabilities: actions pinned to mutable tags (#121), no `permissions:` block
(#123), no SAST (#125), Dependabot blind to npm (#126), the real lockfile
outside CODEOWNERS (#128), no branch protection (#202), no LICENSE file (#130),
no coverage threshold (#131), and the bundled-DB / ACME paths unverified in CI
(#210).

### Malformed / absent input

**Held.** Body caps (`limits.go`), `http.MaxBytesReader` defense-in-depth, panic
recovery with no stack/message leak to clients, `TranslateError` collapsing all
driver errors to a fixed generic message. The exception is the set of import /
sync handler call sites that bypass `writeDomainError` and pass a raw
`err.Error()` into the response body (#119).

## What was not examined

Recorded honestly. Each item below is also filed as a finding so it is closed by
work, not by assertion.

- **No runtime / dynamic testing.** Every finding is from static code reading, one
  `vite build`, `npm audit`, `govulncheck`, and `go run github.com/google/go-licenses`.
  No server was started, no request sent, no browser driven. The Playwright
  benchmark project exists and was not run. "Fails today" claims on the IDOR
  findings are inferred from the absent SQL predicate and the absent
  `UserFromContext` call — high confidence, not executed.
- **Electron packaging / fuses** (#260) — no `@electron/fuses` / `electron-builder`
  / Forge config found; `serverLifecycle.ts`, `serverProcess.ts`,
  `serverBinary.ts`, `singleInstance.ts`, `jobObject.ts`, `healthPoller.ts` not read.
- **Library invitation & membership authorization** (#262) — `AcceptInvitationHandler`,
  `LazyCreateInvitationHandler`, and the membership repository were not traced to
  SQL; a token-forgery or missing "admin of X can only invite to X" check would
  be High.
- **`cmd/pg-supervisor` / Postgres spawn surface** (#264) — argv construction,
  binary-path trust, data-dir permissions, port selection, and the
  `DATABASE_URL`-absent bundled-spawn path.
- **API contract vs. actual response shapes** (#265) — `api/openapi.yaml` was not
  diffed against real responses for over-disclosure; CI stage 6 is a named no-op.
- **`check-parameterized-queries.sh` / `check-import-boundaries.sh`** (#267) — not
  adversarially tested for bypass.
- **Repositories not individually traced to SQL:** `paired_device_repository.go`,
  `pairing_session_repository.go` (encrypted-index lookup),
  `network_settings_repository.go`, `reading_preferences_repository.go`,
  `reading_export_repository.go`, `auth_repository.go` (refresh-token /
  password-reset queries), `source_repository.go` credential encryption at rest.
- **Migrations** not reviewed for nullability / FK / unique-constraint gaps.
- **`internal/jobs/` engine internals** — queue fairness, `CancelJob` / `RetryJob`
  transaction semantics, backoff — only the HTTP-facing surface was read.
- **`internal/importer/extract/*` parsers** — have dedicated `adversarial_test.go`;
  the 250 MiB / entry-count caps were confirmed wired, but the parsers were not
  independently re-audited.
- **`web/src` DOM-XSS sinks** (`dangerouslySetInnerHTML`), token storage location,
  and the reader iframe `sandbox` attribute value were not verified against a
  running renderer.
- **Concurrency** was reasoned about from code structure only, not exercised
  (#137 files the missing sync concurrency test).
- **The `/code-review ultra` cross-check** (maintainer-run) has not yet been
  completed; its findings will be merged into this issue set.
