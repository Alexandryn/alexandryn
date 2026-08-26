<div align="center">

# Alexandryn

**A self-hosted digital library. Runs on your desktop, opens on any device in your home.**

</div>

---

> **Status: pre-alpha. Nothing is implemented yet.**
>
> This repository currently contains the engineering foundation — the
> constitution, the roadmap, and the specifications being written against it.
> There is no application to run. When there is, this notice will say so.

---

## What it is

Alexandryn keeps your books in one place and makes them readable from wherever
you are in the house.

It runs as a desktop application, and that desktop application is also the
server. Open it on your laptop and your tablet can reach the same library over
your home network — same books, same collections, same page you left off on.

Three things stay deliberately separate:

- **Metadata** — what a book is. Titles, authors, editions, subjects, covers.
  Sourced from [Open Library](https://openlibrary.org).
- **Sources** — where files come from. An OPDS server, a folder on a drive, or
  something you add later. You configure these; Alexandryn doesn't ship with
  any.
- **Your library** — what you actually have, how you've organised it, and how
  far you've read.

Keeping those apart is what lets a source disappear without taking your reading
history with it.

## What it isn't

- Not a book store, and not a way to find books that aren't yours to have.
- Not a cloud service. Your library lives on your machine; nothing is uploaded.
- Not exposed to the internet by default. It binds to localhost until you
  decide otherwise, and it will not serve to your network without a login.

## Planned shape

```
                        Alexandryn
                            │
                 ┌──────────┴──────────┐
                 │                     │
             Electron               Web UI
                 │                     │
                 └──────────┬──────────┘
                            │
                      Alexandryn Host
                            │
             ┌──────────────┼──────────────┐
             │              │              │
          Library         Sources        Reader
             │              │              │
             └──────────────┼──────────────┘
                            │
                         Storage
```

| Layer | Technology |
|---|---|
| Interface | React, TypeScript, Tailwind CSS |
| Desktop host | Electron |
| Server | Go |
| Storage | PostgreSQL, self-hosted, local to the host |
| Background work | PostgreSQL-backed job queue (ADR 0014) |
| Packaging | Docker, Docker Compose |

These are the intended choices. Each one is being justified in an architecture
decision record rather than assumed — see `.claude/decisions/`.

## How this project is built

Alexandryn is developed specification-first and test-first. That is not a
stylistic preference; it is written down and enforced.

- **[`.claude/constitution.md`](.claude/constitution.md)** — the rules that
  don't bend. Read this first.
- **[`.claude/roadmap/`](.claude/roadmap/)** — the phases, in dependency order,
  with exit criteria.
- **[`.claude/specs/`](.claude/specs/)** — feature specifications and their
  status.
- **[`.claude/decisions/`](.claude/decisions/)** — architecture decision
  records, including the ones still open.
- **[`.claude/audits/`](.claude/audits/)** — adversarial reviews and findings.

Work moves in one direction:

```
Discover → Spec → Review → Test plan → Red → Implement → Green
        → Refactor → QA → Security audit → Document → Close
```

## Contributing

Start with [CONTRIBUTING.md](CONTRIBUTING.md), then the constitution. The bar
is high on purpose, and it is written down so it isn't arbitrary.

## Security

Please don't open a public issue for a vulnerability. [SECURITY.md](SECURITY.md)
explains how to report one privately, and what's in scope.

## Licence

Not yet chosen — see
[`.claude/decisions/0002-project-licence.md`](.claude/decisions/0002-project-licence.md).
Until one is added, the default applies: all rights reserved. This will be
resolved before the repository becomes public.
