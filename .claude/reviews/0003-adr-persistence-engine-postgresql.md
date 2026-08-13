# Review: ADR 0004 — Persistence engine is self-hosted PostgreSQL

| | |
|---|---|
| **Subject** | `.claude/decisions/0004-persistence-engine-postgresql.md` |
| **Reviewer** | Luann Moreira (pending confirmation — drafted by Claude as a first pass, not self-approved) |
| **Date** | 2026-08-13 |
| **Verdict** | Pending |

## Summary

**Conflict of interest, stated plainly:** I wrote this ADR last session and
am now reviewing my own work. Treat this review as a structured self-check,
not an independent second opinion — the one thing it cannot catch is a blind
spot I already have. The content: PostgreSQL, self-hosted, local to the host;
Supabase is dev/test tooling only, never a shipped dependency. It correctly
rejects cloud Supabase outright as incompatible with the project's stated
premise, and reasonably picks a client-server engine over an embedded one for
planned multi-device LAN access — but the confidence section already flags
that second part as the softer of the two calls, and I agree with that
self-assessment on rereading it.

## Findings

| # | Severity | Area | Finding | Required change |
|---|---|---|---|---|
| 1 | Major | Process | Marked `Accepted` on 2026-08-13 with no recorded review — same violation as 0001/0003, but here it's mine, made worse by also being the one who wrote `reviews/README.md`'s rule that this needs one | Backfilled here; needs your actual read, not my self-assessment, before it's really resolved |
| 2 | Minor | Scope | This ADR bundles two decisions that `decisions/README.md`'s own guidance treats as separable: (a) PostgreSQL vs. an embedded engine, and (b) Supabase's role as dev tooling only, never shipped. They were decided together in one conversation, which is a legitimate reason to keep them in one record, but a future reader trying to reverse just the engine choice (a) has to untangle it from (b) | No change required unless you want them split; noting since `decisions/README.md` says "record the options that lost" per decision, and here two decisions' losing options are interleaved |
| 3 | Minor | Rigor | The SQLite rejection (Option A) leans on "single-writer model is a worse fit" for concurrent LAN access, without checking whether SQLite's WAL mode (concurrent readers + one writer) would actually be insufficient at household scale. The ADR's own Confidence section already flags this as Medium and defers the real check to phase 01 — so this is a gap the ADR is honest about, not one it hides | Already scheduled: phase 01's `architecture-persistence.md` must walk the concrete concurrent-access pattern before this is fully load-bearing, per the ADR's own text |

## Dimensions checked

- [x] **Completeness** — context, decision, three options (including the cloud-Supabase option that needed to be named and rejected explicitly, not just skipped), consequences, reversal, confidence, plus the local-dev-mechanism addendum
- [x] **Ambiguity** — scope is explicit: "everything back-end and data related," matching what you said when I asked
- [x] **Architecture** — consistent with constitution §6 (API-mediated access, no direct LAN-to-DB path) and doesn't touch the metadata/source/Alexandryn boundary (§3), and says so
- [x] **Domain correctness** — explicitly scopes itself to storage for the Alexandryn side only
- [x] **Security** — no hardcoded credential in the end state (the addendum's `.env` + MCP indirection came after I initially hardcoded `postgres:postgres` into a tracked `.mcp.json` and caught it before committing — that mistake isn't visible in the final ADR text, flagging it here so the record is honest about how the addendum came to exist)
- [ ] **Testability** — not applicable to the ADR itself; it doesn't assert behavior, phase 03 will
- [ ] **Accessibility** — not applicable
- [ ] **Observability** — not applicable at this decision's level
- [x] **Maintainability** — reversal cost section is concrete about when this gets expensive (once phase 03 has repositories built against it) rather than a vague "hard to say"
- [x] **Evolution** — correctly identifies what would force the question back open (concurrency assumptions proving wrong, packaging Postgres alongside Electron proving impractical at phase 99)

## Contradictions and gaps

None against the constitution or the other ADRs. One thing worth your eyes
specifically: Option B (cloud Supabase) is rejected using the README's "not a
cloud service" line and constitution §8 — both real, both correctly applied
— but this ADR is also the *only* place that rejection is written down. If
someone edits the README's "What it isn't" section later without checking
ADR 0004, the two could drift apart with nothing forcing them back in sync.

## What I did not review

Everything above is a self-review. I have no way to catch a reasoning error I
already made once while writing the original ADR — that's exactly what an
independent reviewer would be for, and I'm not one here.
