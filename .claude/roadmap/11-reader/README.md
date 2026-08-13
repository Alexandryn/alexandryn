# Phase 11 — Reader

*Outline — expanded to a full phase document when phases 06, 10 close.*

| | |
|---|---|
| **Status** | Not started |
| **Depends on** | Phase 06, Phase 10 |
| **Blocks** | 14 |

## Objective

The in-browser reader: EPUB rendering, typography preferences, position
tracking, bookmarks, highlights — tested against a real corpus of imported
files, with EPUB content sandboxed as hostile by default.

## Scope

**In**

- EPUB rendering (paginated and scrolling), typography/theme preferences
- Reading position, bookmarks, highlights, table of contents
- Sandboxed rendering — no script execution from EPUB content

**Out**

- Cross-device sync of position — phase 14.

## Exit criteria

- [ ] Renders real imported EPUBs correctly across screen sizes
- [ ] Position reliably saved and restored
- [ ] EPUB sandbox verified against a script-injection test case
- [ ] Full keyboard and screen-reader operability
- [ ] Maintainer approval recorded
