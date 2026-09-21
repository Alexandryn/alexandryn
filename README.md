<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/assets/banner-dark.png">
  <source media="(prefers-color-scheme: light)" srcset=".github/assets/banner-light.png">
  <img alt="Alexandryn — A self-hosted digital library" src=".github/assets/banner-light.png" width="100%">
</picture>

# Alexandryn

**A self-hosted, local-first personal digital library. Runs on your desktop, opens on any device in your home.**

[![Release: v1.0.0](https://img.shields.io/badge/release-v1.0.0-41608F.svg)](https://github.com/Alexandryn/alexandryn/releases)
[![License: AGPL v3](https://img.shields.io/badge/license-AGPL_v3-blue.svg)](LICENSE)
[![Local First](https://img.shields.io/badge/architecture-local--first-B07C4F.svg)](#the-three-pillars)
[![Zero Telemetry](https://img.shields.io/badge/telemetry-zero-4A8A68.svg)](#strict-privacy--security-model)
[![Web Reader](https://img.shields.io/badge/reader-EPUB_%26_PDF-863BFF.svg)](#distraction-free-reading-engine)

[**Website**](https://alexandryn.github.io/website/) · [**Documentation**](https://alexandryn.github.io/docs/) · [**Architecture Decisions**](docs/decisions/) · [**Specifications**](docs/specs/)

</div>

---

## What it is

Alexandryn keeps your book collection in one sovereign place and makes it readable from wherever you are in the house.

It operates on a dual hosting topology:
1. **Desktop Host**: An all-in-one desktop application (macOS, Windows, Linux) that is also a local background server. Open it on your laptop, and your tablet or phone can reach the exact same library over your home network — same books, same collections, same page you left off on.
2. **Headless Home Server**: A hardened Docker container stack designed for homelabs, home servers, and NAS appliances.

Unlike monolithic eBook managers or cloud reading platforms, Alexandryn is designed around a single non-negotiable rule: **your reading history and collections belong to you, not to a service, and not even to the storage drive where your book files live.**

---

## The Three Pillars

Alexandryn maintains a strict, architectural decoupling between three domains:

```
┌─────────────────────────┐     ┌─────────────────────────┐     ┌─────────────────────────┐
│        METADATA         │     │         SOURCES         │     │       YOUR LIBRARY      │
│  What a book is         │     │  Where files come from  │     │  Your personal record   │
│                         │     │                         │     │                         │
│ • Open Library cache    │     │ • Local disk folders    │     │ • Reading progress      │
│ • Titles, authors, ISBN │     │ • Calibre directories   │     │ • Position & bookmarks  │
│ • Editions, covers      │     │ • OPDS 1.2 catalogs     │     │ • Custom collections    │
│ • Subject classification│     │ • Unaltered file storage│     │ • Paired device state   │
└─────────────────────────┘     └─────────────────────────┘     └─────────────────────────┘
```

Keeping these apart is what lets a file source disconnect, change, or disappear without ever taking your reading history or shelves with it:

- **Metadata**: Alexandryn queries [Open Library](https://openlibrary.org) to enrich your catalog with high-resolution covers, work-level summaries, and bibliographic records, cached in your local PostgreSQL database.
- **Sources**: Points to folders on your drives or remote OPDS servers. Alexandryn never copies, relocates, or modifies your original files.
- **Your Library**: How far you have read, your bookmarks, your reading history, and how you have organized your shelves stay permanently anchored in your local database.

---

## Meet Alex, the Library Guardian

<img src=".github/assets/mascot.svg" align="right" width="160" alt="Alex the Cat — Alexandryn Mascot" />

**Alex** is the feline guardian of Alexandryn.

Inspired by the historic Library of Alexandria and the quiet companionship of library cats throughout centuries of book care, Alex sits watchful and composed atop an open codex.

Alex symbolizes the project's core philosophy:
- **Vigilance**: Keeping guard over your reading privacy — zero telemetry, zero surveillance, and no third-party tracking.
- **Permanence**: Ensuring your digital library endures across hardware upgrades, network reorganizations, and storage changes.
- **Quiet Sovereignty**: Software that stays out of your way, running quietly in the background without nagging popups, subscriptions, or dark patterns.

---

## Core Capabilities

### Native Desktop Host
- Packaged with Electron with strict security fuses enabled (`RunAsNode` off, cookie encryption on, embedded asar integrity).
- Embeds a high-performance Go backend and manages its own PostgreSQL supervisor process — zero external database configuration or CLI setup needed for desktop users.

### Responsive LAN Web Reader
- Fully responsive interface built with React, TypeScript, and Tailwind CSS.
- Automatically accessible from any browser on your home Wi-Fi (`http://<host-ip>:8080`).
- Touch-optimized for iPad, Android tablets, Kindle Fire, and mobile browsers.

### Distraction-Free Reading Engine
- High-fidelity EPUB and PDF reader powered by Foliate-JS.
- Classical literary typography featuring Newsreader and Georgia serif typefaces.
- Configurable font sizing, line height, column widths, margins, and dark/light/sepia color palettes.
- Continuous reading position tracking down to the exact paragraph and CFI.

### Cross-Device Reading Progress Synchronization
- Real-time reading position sync across paired devices via Server-Sent Events (SSE).
- Seamless QR-code pairing flow to authorize new phones and tablets on your local network.

### Pluggable Ingestion Pipeline
- Watch folders for automated indexing of new `.epub` and `.pdf` files.
- Direct read adapter for Calibre library directories.
- Native OPDS 1.2 catalog ingestion.

### Strict Privacy & Security Model
- Binds strictly to `127.0.0.1` (loopback) by default.
- LAN exposure requires authenticated user accounts and cryptographic session tokens.
- Public/remote reachability enforces fail-closed TLS (ADR 0017).
- Absolutely zero telemetry, analytics, or phone-home pings.

---

## What it isn't

- **Not a bookstore**: Alexandryn provides no mechanism to discover or acquire files you do not already own.
- **Not a cloud locker**: Your library never uploads your reading data, notes, or files to external servers.
- **Not an internet-exposed service by default**: Alexandryn will not serve outside your local host without deliberate configuration, a password-protected account, and TLS.

---

## Quickstart

### Option A: Desktop Application (Recommended)

Download the installer for your operating system from the [Latest Release](https://github.com/Alexandryn/alexandryn/releases/latest):

- **macOS**: `Alexandryn-1.0.0.dmg` (Apple Silicon & Intel)
- **Windows**: `Alexandryn-Setup-1.0.0.exe`
- **Linux**: `Alexandryn-1.0.0.AppImage` or `alexandryn_1.0.0_amd64.deb`

Launch the app. Alexandryn will initialize its embedded database, start its background host, and open the library interface.

### Option B: Docker Compose (Home Server / NAS)

For homelabs, Raspberry Pis, or headless servers, use the production Docker Compose stack:

```yaml
services:
  alexandryn:
    image: ghcr.io/alexandryn/alexandryn:1.0.0
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      - BIND_ADDRESS=0.0.0.0:8080
      - DATABASE_URL=postgres://alexandryn:secret_password@postgres:5432/alexandryn?sslmode=disable
      - OPEN_LIBRARY_USER_AGENT=Alexandryn/1.0.0 (your-email@example.com)
    volumes:
      - alexandryn_data:/home/app/.config/alexandryn
      - /path/to/your/books:/books:ro
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      - POSTGRES_DB=alexandryn
      - POSTGRES_USER=alexandryn
      - POSTGRES_PASSWORD=secret_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U alexandryn -d alexandryn"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  alexandryn_data:
  postgres_data:
```

Run:
```bash
docker compose up -d
```
Then navigate to `http://<server-ip>:8080` to complete the initial administrator setup.

---

## Architecture

```
                                  Alexandryn
                                      │
                   ┌──────────────────┴──────────────────┐
                   │                                     │
           Desktop Host (Electron)             Web & Mobile (Browser)
                   │                                     │
                   └──────────────────┬──────────────────┘
                                      │
                             Alexandryn Host (Go)
                        REST API · SSE · Auth · Engine
                                      │
                   ┌──────────────────┼──────────────────┐
                   │                  │                  │
              Library Core      Sync & Pairing     Reader Engine
                   │                  │                  │
                   └──────────────────┼──────────────────┘
                                      │
         ┌────────────────────────────┼────────────────────────────┐
         │                            │                            │
    PostgreSQL                 Source Adapters                Open Library
  (Data & Queue)            (OPDS / Local Folders)          (Metadata Cache)
```

| Layer | Technology | Responsibilities |
| :--- | :--- | :--- |
| **Interface** | React, TypeScript, Tailwind CSS | Library grid, search, collections, settings, book detail, reader view |
| **Desktop Shell** | Electron | Window management, menu bar daemon, process supervisor |
| **Application Server** | Go | REST endpoints, SSE progress stream, auth token verification, background jobs |
| **Database** | PostgreSQL | Multi-tenant schema, reading progress state, job queue, metadata cache |
| **Reader** | Foliate-JS, Web APIs | Client-side EPUB rendering, PDF viewport, typography reflow |
| **Packaging** | Docker, electron-builder | Multi-arch container images, native desktop installers |

---

## Engineering Quality & Rigour

Alexandryn was developed from day one with an uncompromising specification-first and test-driven engineering discipline:

- **100% Test-First**: Every backend endpoint, domain transition, and frontend screen is covered by automated unit, integration, or end-to-end tests.
- **The Constitution ([`docs/constitution.md`](docs/constitution.md))**: Twelve non-negotiable architectural invariants:
  - Input validation with strict shape and timeout limits (§4).
  - Domain separation between metadata, storage, and user libraries (§3).
  - Tenant-scoped queries preventing horizontal access violations (§6).
  - Web Content Accessibility Guidelines (WCAG 2.1 AA) compliance across all viewports (§7).
  - Zero credential or reading content logging (§8).
- **Adversarial security review**: every major milestone underwent multi-pass adversarial security sweeps before closure; the results are in the repository history.
- **Architecture Decision Records ([`docs/decisions/`](docs/decisions/))**: All significant decisions, from database engine selection to licensing, are formally recorded with context and consequences.

---

## Project Ecosystem

- **[Alexandryn Core](https://github.com/Alexandryn/alexandryn)** — The primary desktop and server application repository.
- **[Documentation](https://alexandryn.github.io/docs/)** — Detailed guides on self-hosting, administration, pairing, network configuration, and reverse proxies.
- **[Website](https://alexandryn.github.io/website/)** — The official public landing page.
- **[OpenAPI Contract](architecture-contracts.md)** — Machine-readable API specification.

---

## Contributing

We welcome contributions that respect the project's quality bar. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and [`docs/constitution.md`](docs/constitution.md) before opening a pull request.

---

## Security

Please do not open public GitHub issues for security vulnerabilities. Review [SECURITY.md](SECURITY.md) for our disclosure policy, PGP keys, and private reporting instructions.

---

## Licence

Licensed under the **[GNU Affero General Public License v3.0 or later](LICENSE)** ([ADR 0002](docs/decisions/0002-project-licence.md)).

Because Alexandryn is self-hosted software, the AGPL ensures that anyone offering modified versions over a network must share their source code with their users, preserving software freedom for all readers.
