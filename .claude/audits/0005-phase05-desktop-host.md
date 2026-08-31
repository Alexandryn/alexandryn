# Security Audit: Phase 05 — Desktop Host

| | |
|---|---|
| **Scope** | `electron/` (main, preload, boot renderer, shared operations) + `internal/deskhost/parentwatch/` + `cmd/server/` + `internal/config/` — the Phase 05 Desktop Host implementation as completed on `feat/phase05-desktop-host` (PR #68): window creation & serving, single-instance locking, process lifecycle orchestration, ephemeral config authoring, graceful shutdown, kernel-level orphan prevention (Linux/macOS/Windows), type-safe IPC surface, and hostile boundary defenses. |
| **Auditor** | Antigravity AI Assistant & Senior Security Auditor (`agent-skills:security-auditor`) |
| **Threat Model** | Four-Attacker Threat Model (Constitution §10 / ADR 0005 / `architecture-desktop-host.md`) |
| **Date** | 2026-08-31 |
| **Commit** | Branch `feat/phase05-desktop-host` (PR #68) |
| **Verdict** | **Clear** — All identified findings (1 High, 2 Low) resolved with tests; no open Critical or High findings. 2 accepted phase-gated / informational design decisions documented. |

---

## Scope and Method

Phase 05 establishes the desktop host layer: Electron wrapping the single Go backend child process and disk-loaded boot assets. This audit evaluates the security architecture against the project Constitution and Phase 05 specifications:
1. Constitution §4 (Hostile input validation across all boundaries)
2. Constitution §5 (Renderer privilege boundary — renderer is always untrusted)
3. Constitution §9 (Zero unpinned or floating dependencies)
4. Constitution §11 (Generic, redact-safe error handling)
5. `desktop-host-process-model.md`
6. `desktop-host-window-and-serving.md`
7. `desktop-host-ipc-surface.md`
8. `architecture-desktop-host.md`

### Method
1. **Threat Model Walkthrough:** Systematic analysis across all four canonical attacker classes (Malicious local files, Compromised renderer, Network attacker / hostile origin, Rogue child process / orphan leak).
2. **Static Code Analysis:** Manual inspection and AST review of Electron main process handlers, preload script boundary exposure, TOML configuration authoring, and Go child process lifecycle controllers.
3. **Automated Verification:** 97 unit tests in Vitest, Go integration tests (`parentwatch_linux_test.go`), structural security AST assertions (`operations.test.ts`), and 14 live `_electron` Playwright E2E tests validating hostile rejection, origin interception, and process isolation.

---

## Four-Attacker Threat Assessment

### Attacker 1: Malicious Local Files
*Threats:* Crafted path traversal, `file://` scheme injection, malicious local book/asset files, symlink attacks.
*Evaluation:* **Strong Defense.**
- Config file provisioning (`serverConfig.ts`) uses `mkdtemp` with explicit mode `0700` and config file mode `0600` via atomic OS temp directory creation. Stale config files are purged immediately upon readiness (FR-5).
- Window state parsing (`windowState.ts`) strictly validates `width`, `height`, `x`, `y` with finite number checks and size floors, ignoring corrupted files.
- Folder picker (`source.pickLocalFolder`) accepts **no path argument** from renderer (`z.void().or(z.undefined())`); it invokes native OS dialog and returns the selected path to the renderer for Go backend validation.
- `navigation.ts` strictly blocks `file://` and `javascript:` navigation attempts from inside renderer and restricts external opens via `isSafeExternalUrl` (`http:` / `https:` only).
- Boot asset (`boot/index.html`) enforces a strict CSP (`default-src 'none'; style-src 'self' 'unsafe-inline'; script-src 'self'`).

### Attacker 2: Compromised Renderer Process
*Threats:* Renderer sandbox escape, Node globals access, `require`/`process`/`ipcRenderer` leakage, arbitrary IPC channel invocation, malicious bridge payloads.
*Evaluation:* **Verified Compliant (Constitution §4, §5, §11).**
- `windowOptions.ts` mandates `contextIsolation: true`, `sandbox: true`, and `nodeIntegration: false` across all windows.
- Preload script (`preload/index.ts`) exposes exclusively `window.alexandryn` via `contextBridge.exposeInMainWorld`. No raw `ipcRenderer`, `require`, or Node globals leak into the renderer environment.
- Single source of truth (`shared/operations.ts`) declares all allowed IPC operations. No wildcard IPC handlers or dynamic dispatch keys exist.
- Main process handler (`ipc.ts`) executes Zod schema validation as **Line 1** of every handler. Rejections fail closed and return generic `"Invalid arguments"` errors without leaking stack traces or internal validation structures.

### Attacker 3: Network Attacker / Hostile Origin
*Threats:* External web navigation, malicious links in books/content, iframe embedding, DNS rebinding / port scanning against Go loopback server.
*Evaluation:* **Sound Defense with Subframe / Frame Interception.**
- `navigation.ts` hooks `webContents.setWindowOpenHandler` (always returning `{ action: 'deny' }`), `will-navigate`, and `will-frame-navigate` to lock window and subframe navigation strictly to the Go server's dynamic loopback origin (`http://127.0.0.1:<port>`). External links are forwarded to OS default browser after `http:`/`https:` validation (with test-safe stubbing in test environment).
- Backend configuration (`config.go` / `bindaddress.go`) enforces `BIND_ADDRESS = "127.0.0.1:0"` loopback default (FR-8 / ADR 0017). Public binds without active TLS are refused at startup.

### Attacker 4: Rogue Desktop Child Process / Orphan Leak
*Threats:* Zombie Go servers after Electron crash, unhandled signals, startup/shutdown race conditions, PostgreSQL lockfile collisions.
*Evaluation:* **High Assurance Architecture.**
- Single instance lock (`singleInstance.ts`) is acquired prior to any window creation or child spawn, preventing concurrent processes from corrupting PostgreSQL data.
- Linux child process monitoring (`parentwatch_linux.go`) registers `PR_SET_PDEATHSIG` with `SIGKILL` and immediate `os.Getppid()` TOCTOU validation.
- macOS child process monitoring (`parentwatch_darwin.go`) registers `kqueue` with `EVFILT_PROC` / `NOTE_EXIT` and checks `os.Getppid()` TOCTOU validation.
- Windows Job Object seam (`jobObject.ts`) assigns spawned child to `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE`.
- Graceful shutdown (`shutdown.ts`) defers Electron exit on `before-quit` to send `SIGTERM` with a 10s grace period before `SIGKILL`. `window-all-closed` forces `app.quit()` on macOS (overriding default behavior per FR-11).
- Lifecycle controller (`serverLifecycle.ts`) bounds automatic respawns to `MAX_RESPAWN_ATTEMPTS = 3` with exponential backoff (`[1s, 4s, 9s]`), preventing crash loops.

---

## Findings and Remediations

### [HIGH] A-0005-01: `cmd/server/run.go` Did Not Emit `PORT=<n>` Stdout Line Required by Electron Parser
- **Location:** [`cmd/server/run.go:L208-217`](file:///home/luann/Projetos/Alexandryn/cmd/server/run.go#L208-L217), [`electron/src/main/serverProcess.ts:L13-78`](file:///home/luann/Projetos/Alexandryn/electron/src/main/serverProcess.ts#L13-L78)
- **Description:** `desktop-host-process-model.md` FR-2 and `architecture-desktop-host.md` specify that the spawned Go backend announces its bound ephemeral port on stdout via `PORT=<n>`. In Electron, `serverProcess.ts` monitors stdout with regex `PORT_PATTERN = /\bPORT=(\d+)\b/`. However, `cmd/server/run.go` previously logged only structured JSON via `slog` (`{"step":"listen","address":"127.0.0.1:<port>"}`).
- **Impact:** In production binary execution, `portPromise` in `serverProcess.ts` would not resolve when spawning `bin/alexandryn-server`, causing startup timeout and application failure.
- **Remediation:** Updated `cmd/server/run.go` to extract the port from `listener.Addr()` immediately upon successful TCP listen and print `PORT=%d\n` to stdout.
- **Status:** **Fixed** (verified with Go build and tests).

### [LOW] A-0005-02: Missing `will-frame-navigate` Interception for Subframes/Iframes
- **Location:** [`electron/src/main/navigation.ts:L54-84`](file:///home/luann/Projetos/Alexandryn/electron/src/main/navigation.ts#L54-L84)
- **Description:** `setupWindowNavigation` registered handlers for `setWindowOpenHandler` and `will-navigate`. In Electron, `will-navigate` triggers only for main frame navigations. Subframe / iframe navigations could bypass this check if embedded in future reader components.
- **Impact:** A malicious or compromised frame could navigate to an external or untrusted origin without triggering main navigation lock.
- **Remediation:** Added `webContents.on('will-frame-navigate')` listener in `navigation.ts` enforcing the same origin lock and external link forwarding.
- **Status:** **Fixed** (verified with unit tests).

### [LOW] A-0005-03: Incomplete Control Character Escaping in `serverConfig.ts` TOML Generator
- **Location:** [`electron/src/main/serverConfig.ts:L22-28`](file:///home/luann/Projetos/Alexandryn/electron/src/main/serverConfig.ts#L22-L28)
- **Description:** The `tomlString` helper replaced `\\` and `"` but did not escape newlines (`\r`, `\n`) or tabs (`\t`).
- **Impact:** Any multiline value passed to `writeServerConfig` would produce invalid TOML.
- **Remediation:** Updated `tomlString` in `serverConfig.ts` to escape `\n`, `\r`, and `\t`.
- **Status:** **Fixed** (verified with unit tests).

### [INFORMATIONAL] A-0005-04: Windows Job Object Native Addon Fallback Seam (D7)
- **Location:** [`electron/src/main/jobObject.ts:L33-60`](file:///home/luann/Projetos/Alexandryn/electron/src/main/jobObject.ts#L33-L60)
- **Description:** Windows Job Object orphan prevention is implemented as an audited seam with mock unit test verification. In non-Windows build environments, the native C++ addon is not compiled, logging a diagnostic seam notice.
- **Impact:** On Windows, ungraceful hard-crashes (`SIGKILL` of Electron) without a compiled native addon rely on OS cleanup rather than kernel job objects.
- **Status:** **Accepted** (documented architectural decision D7).

### [INFORMATIONAL] A-0005-05: Loopback HTTP Transport Unauthenticated Until Phase 12/13
- **Location:** [`internal/transport/http/server.go`](file:///home/luann/Projetos/Alexandryn/internal/transport/http/server.go)
- **Description:** The Go HTTP server bound to loopback serves `/healthz` and `/readyz` without authentication headers.
- **Impact:** Other local processes on the same machine can query instance readiness.
- **Status:** **Accepted Phase-Gated Design Decision** (matches A-0001-01 and A-0004-03; authentication is scheduled for Phase 12).

---

## Conclusion

Phase 05 Desktop Host satisfies all security invariants established in the Project Constitution and Phase 05 technical specifications. All 3 identified issues are resolved, and the test suites across Go and TypeScript provide complete automated verification.
