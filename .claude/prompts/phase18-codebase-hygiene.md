# Phase 18 — Human Codebase Cleanup & Release Hygiene
> **Master Execution Prompt for Alexandryn**
> *Copy and paste this prompt directly into a new session to execute Phase 18.*

---

You are executing **Phase 18 — Human Codebase Cleanup & Release Hygiene** for **Alexandryn** (`github.com/Alexandryn/alexandryn`).

Read [`.claude/constitution.md`](.claude/constitution.md), [`CLAUDE.md`](CLAUDE.md), and [`.claude/roadmap/18-codebase-hygiene/README.md`](.claude/roadmap/18-codebase-hygiene/README.md) before starting.

---

## Operating Principles & Skills

1. **Caveman Communication Register**:
   - Activate `/caveman full` immediately. Keep all conversational interaction with the maintainer terse, high-signal, and token-efficient.
   - **Crucial Boundary (`CLAUDE.md` § Scope)**: Chat is caveman, but **all code comments, commit messages, PR descriptions, documentation, and user-facing copy MUST be written in polished, standard, professional human prose**. Never emit caveman fragments inside source files or commit logs.

2. **Active Skills & Delegation**:
   - Use `/agent-skills:using-agent-skills` to coordinate cleanup activities.
   - Apply `/agent-skills:code-simplification` to simplify confusing control flow and eliminate redundant state.
   - Apply `/caveman:safe-refactor` when restructuring code blocks to guarantee behavior preservation bracketed by automated tests.
   - Apply `/agent-skills:code-review-and-quality` for evaluating human readability and multi-axis code hygiene.
   - Delegate targeted, bounded edits (1–2 files) to `cavecrew-builder` to keep the main conversation context clean.
   - Delegate pre-merge human-readability and regression checks to `code-reviewer`.

3. **Inviolable Engineering Safeguards (Constitution §1–§12)**:
   > **Simplicity does NOT mean removing useful engineering safeguards.**
   - **Never delete or relax input validation**: Every external input (OPDS responses, book files, network payloads, IPC messages) must retain its size limit, shape check, and timeout (§4).
   - **Never collapse domain boundaries**: Metadata (Open Library), Sources, and Alexandryn domain aggregates must remain strictly separated (§3). Do not combine types or handlers across boundaries for convenience.
   - **Never weaken Electron privilege boundaries**: Context isolation, sandboxing, disabled Node integration, and strict preload IPC argument validation in the main process are immutable (§5).
   - **Never alter network security invariants**: Loopback binding by default and ADR 0017 fail-closed TLS guarantees are non-negotiable (§6).
   - **Never remove tenant scoping**: Every repository query and handler serving user data must keep its `user_id` and `library_id` predicates (§3, §6).
   - **Never weaken accessibility**: Semantic HTML, keyboard operability, ARIA attributes, VisuallyHidden announcements, and focus management are permanent (§7).
   - **Never touch log redaction**: Structured logs must never leak tokens, credentials, home paths, or book reading contents (§8).
   - **Never add dependencies**: Zero new dependencies (§9).

---

## Primary Objective

Systematically search and clean the repository of all **development-process residue**, **conversational AI artifacts**, and **unnecessary comments**, transforming the codebase into an intentionally maintained, self-explaining, mature open-source product.

### The Standard
> **"Every remaining comment must earn its existence by explaining non-obvious WHY, not WHAT."**
> Code should communicate its intent through clear naming, small focused functions, and strong typing.

---

## Targeted Alexandryn Residue Inventory

Search for and eliminate the following specific patterns across Go, TypeScript, React, Electron, SQL, and Shell scripts:

### 1. Development Phase Narration
- Comments citing roadmap phases:
  - `// Package sources is phase 08's source-adapter layer`
  - `// Phase 10: Import pipeline...`
  - `// Phase 11: the reader...`
  - `// Phase 13 network keys...`
  - `// Phase 15: Observability...`
  - `// Phase 16 (#116): FinishedWorksHandler...`
  - `// Phase 17 Gate 0...`
- *Action*: Remove phase annotations. If the comment describes durable architectural rationale, rewrite it without referencing developmental phases.

### 2. Spec & Requirement Tokens
- Functional requirement IDs and milestone markers:
  - `(FR-1, FR-9)`, `(FR-4 amendment, ADR 0028)`
  - `(backend-test-harness.md FR-8)`, `(frontend-design-tokens.md FR-2)`
  - `(desktop-host-process-model.md FR-1)`
  - Spec tracking tokens: `(P0, P4-G, T24, T26, D2, D3, D5)`
- *Action*: Strip requirement IDs from code comments. The code should stand on its own merits.

### 3. Bug-Tracker & Audit Citations
- Audit and review issue references:
  - `(audit 0016 #205)`, `(audit 0017 #325)`, `(AUDIT-0012-C2)`, `(review 0050)`
- *Action*:
  - If the comment explains a **critical, non-obvious engineering constraint** (e.g., token type verification or tenant-scoped queries), rewrite it as timeless engineering rationale (e.g., `"Assert token type explicitly to prevent privilege escalation from tokens minted for pairing or MFA"`).
  - If the comment merely points to an issue or review number without durable engineering value, remove it.

### 4. Conversational AI Residue & Meta-Narratives
- References to AI workflows or instructions:
  - `CLAUDE.md token-type Reflex`, `Claude was instructed to...`, prompt citations, agent directives.
  - Instructions written to guide an AI agent rather than a human software engineer.
- *Action*: Delete completely.

### 5. Paraphrasing & Syntax-Explaining Comments
- Comments that merely narrate what the next line of code does:
  - `// Check if user is authenticated` -> `if isAuthenticated {`
  - `// Return nil error` -> `return nil`
  - `// Set timeout to 5 seconds`
  - `// User represents a user` (stuttering Go docstrings)
  - JSDoc blocks on React components repeating TypeScript props already declared in interfaces.
- *Action*: Remove.

### 6. Dead Code & Obsolete Markers
- Unactionable TODOs, commented-out debug code, obsolete temporary workarounds, and migration notes that have long passed.
- *Action*: Remove cleanly.

---

## Comment Quality Standard

Before keeping any comment, evaluate it against these five questions:

1. **Does the code already explain this?**
   - If yes: Remove the comment.
2. **Does the comment explain WHY rather than WHAT?**
   - Retain only non-obvious architectural, algorithmic, or security rationale.
3. **Would a future maintainer need this information to avoid breaking something?**
   - If not: Remove it.
4. **Can the code express this better through naming, small functions, or types?**
   - If yes: Refactor the code for clarity and delete the comment.
5. **Is the comment describing project history?**
   - If yes: Remove it. Git history and ADRs already preserve project evolution.

---

## Scope by Subsystem

### 1. Go Backend (`cmd/`, `internal/`)
- Audit `cmd/server/`, `cmd/pg-supervisor/`, and all `internal/` packages (`domain`, `persistence/postgres`, `transport/http`, `sources`, `metadata`, `jobs`, `reader`, `auth`, `sync`, `observability`, `testutil`).
- Ensure Go doc comments on exported symbols describe behavior, constraints, and contracts concisely without stuttering or phase cross-references.
- Simplify convoluted control flow, unnecessary error wrapping, or redundant nil-checks.

### 2. Web Frontend (`web/src/`)
- Audit `components/`, `hooks/`, `screens/`, `app/`, `stores/`, `mocks/`, and Storybook stories.
- Strip redundant JSDoc comments repeating TypeScript types.
- Simplify component state: replace redundant `useState` + `useEffect` synchronizations with direct derived values.
- Verify styling adheres to tokens without inline hacks.

### 3. Desktop Host (`electron/src/`)
- Audit `main/`, `preload/`, and `renderer/boot/`.
- Ensure comments around the IPC bridge, context isolation, and window lifecycle focus strictly on security guarantees.

### 4. Migrations & Infrastructure
- `internal/persistence/postgres/migrations/`: Clean active SQL comments of conversational process notes, while preserving schema table/column comments needed by database operators.
- `scripts/*.sh`: Clean shell guard scripts and their test harnesses of developmental phase tags while retaining usage instructions.
- `.github/workflows/ci.yml`: Clean CI step annotations of historical issue citations, keeping clear explanations of why specific caching, display servers (xvfb), or dependencies are structured as they are.
- `Dockerfile` & `docker-compose.yml`: Ensure comments explain runtime configuration cleanly.

### 5. Documentation (`README.md`, `CONTRIBUTING.md`, `SECURITY.md`)
- Ensure docs speak to human developers, self-hosters, and open-source contributors.
- Eliminate internal prompt guides or development-phase tracking that belongs only in `.claude/`.

---

## Verification & Quality Gates

After every subsystem cleanup, run local tests. Before completing Phase 18, the entire repository quality battery must pass 100% green without regressions.

### 1. Shell Guards & Static Checks
```bash
bash scripts/check-gofmt.sh .
bash scripts/check-import-boundaries.sh .
bash scripts/check-parameterized-queries.sh .
bash scripts/check-compose-published-port.sh .
bash scripts/check-user-scoped-reading.sh .
bash scripts/check-license.sh .
bash scripts/check-coverage.sh .
```

### 2. Go Backend Quality Battery
```bash
go vet ./...
golangci-lint run ./...
go test -race -coverprofile=coverage.out ./...
go test -race -tags=integration ./...
go test -race -v ./internal/testutil/contracttest/...
govulncheck ./...
gosec -quiet -severity high -confidence high ./...
```

### 3. Frontend Quality Battery
```bash
npm run -w web build
npm run -w web lint
npm run -w web test
npm run -w web check:token-styling
npm run -w web check:a11y-tabindex
npm run -w web check:a11y-hidden-text
npm run -w web check:bundle-size
npm run -w web check:radix-dedup
npm run -w web check:dist-secrets
npm run -w web check:dist-msw
npm run -w web build-storybook
npx playwright test
```

### 4. Desktop Host Quality Battery
```bash
npm run -w @alexandryn/desktop build
npm run -w @alexandryn/desktop typecheck
npm run -w @alexandryn/desktop lint
npm run -w @alexandryn/desktop test
xvfb-run -a npm run -w @alexandryn/desktop test:e2e
```

### 5. Container Target Test
```bash
docker compose --profile bundled-db up --build --wait --wait-timeout 120
docker compose --profile bundled-db down -v
```

---

## Execution Workflow

1. **Phase Discovery & Grep Audit**:
   - Run case-insensitive grep across the repository for `Phase `, `FR-`, `audit `, `CLAUDE.md`, `TODO`, `FIXME`, and obvious comment markers.
   - Group findings by subsystem.
2. **Backend Cleanup**:
   - Clean `internal/` and `cmd/` packages systematically.
   - Run Go unit and integration tests after each package to ensure zero behavioral change.
3. **Frontend Cleanup**:
   - Clean `web/src/` components, hooks, and tests.
   - Run `npm test`, `npm run build`, and token/a11y checks.
4. **Desktop Host Cleanup**:
   - Clean `electron/src/`.
   - Run typecheck, lint, and Electron tests.
5. **Infrastructure & Documentation**:
   - Clean scripts, CI workflows, Dockerfiles, and documentation.
6. **Full Battery Verification**:
   - Run the full suite of automated tests and linters.
7. **Final Review & Report**:
   - Perform a human-readability review simulating a new engineer joining the project.
   - Generate the final closure report.

---

## Final Closure Report Format

At the conclusion of Phase 18, report back under the standard Alexandryn headings (`CLAUDE.md` line 268) followed by the cleanup breakdown:

```markdown
### Changed
[Summary of packages and files cleaned]

### Verified
[Summary of test suites, linters, shell guards, and builds executed]

### Risks
[Analysis of any subtle areas modified and proof of behavior preservation]

### Security
[Confirmation that all input validations, tenant scopes, Electron fuses, and TLS conditions remain untouched]

### QA
[A11y, cross-browser, and desktop test results]

### Architecture
[Confirmation that domain boundaries remain strictly separated]

### Next
[Hand-off state for Phase 99 (Release)]

---

### Cleanup Breakdown
- **Comments Removed**: [Summary of categories pruned and rough count]
- **AI & Development Artifacts Removed**: [Summary of phase markers, FR tags, and audit citations eliminated]
- **Code Simplified**: [Key areas where control flow or state was simplified for readability]
- **Documentation Changed**: [Updates to public-facing documentation]
- **Retained Intentional Comments**: [Key categories of comments deliberately preserved and why]
- **Release Impact**: [Confirmation of repo readiness for packaging in Phase 99]
```
