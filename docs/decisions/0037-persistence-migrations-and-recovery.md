# 0037. Persistence, migrations, restart durability, and backup/recovery strategy

| | |
|---|---|
| **Status** | Accepted |
| **Date** | 2026-09-28 |
| **Deciders** | Luann Moreira |
| **Supersedes** | — |
| **Superseded by** | — |

## Context

Alexandryn supports two deployment models:
1. **Desktop-managed hosting:** Electron AppImage manages the Go server and a bundled PostgreSQL instance.
2. **Docker-managed hosting:** Docker runs the Go server alongside PostgreSQL and NGINX services.

`architecture-persistence.md` (FR-1 through FR-10) established core persistence policies (PostgreSQL engine, forward-only migrations at startup, fail-closed corruption handling, and loopback binding) but explicitly left whole-system backup/restore ownership open.

During Phase 6 (Persistence, Migrations & Recovery), an audit of storage, migration safety, restart behavior, and disaster recovery was conducted to ensure both hosting models prevent user data loss across upgrades, restarts, and recovery scenarios.

## Decision

### 1. Storage Topology & Persistence Requirements

Storage is split into distinct categories, each with defined persistence guarantees:

* **Database Records (Relational Schema):**
  * *Electron-managed:* Stored in OS-standard per-user configuration directory (`~/.config/alexandryn/pgdata` on Linux, `%APPDATA%/alexandryn/pgdata` on Windows, `~/Library/Application Support/alexandryn/pgdata` on macOS) with strict `0700` directory permissions.
  * *Docker-managed:* Stored in the named Docker volume `postgres-data` mounted to `/var/lib/postgresql/data`.
* **Master Cryptographic Key:**
  * Single 32-byte AES-256 key stored in `source-credentials.key` (mode `0600`).
  * In Electron: `~/.config/alexandryn/source-credentials.key`.
  * In Docker: named volume `app-data` mounted to `/home/app/.config/alexandryn`.
  * All cryptographic secrets (source credentials, JWT signing, TOTP master secret, device pairing keys, enrolment grants, reader grants) derive subkeys from this master key via HKDF.
  * *Invariance:* The master key must persist across restarts. If the key file is missing, the server logs a warning and generates a replacement. If a key file exists but is corrupted (not 32 bytes), the server fails closed and refuses to overwrite it.
* **Book Files & Content:**
  * Books reside in user-configured local folder sources or remote OPDS servers. Alexandryn does not store duplicate book blobs inside PostgreSQL.
  * In Docker, host directories containing book collections are mounted into the backend container (e.g. `/path/to/books:/books:ro`).
* **Application Configuration:**
  * Resolution precedence: compiled defaults < optional `config.toml` < environment variables.
  * In Electron, configuration is authored into an ephemeral, secure temporary directory (`mkdtemp`, mode `0700`, file mode `0600`) and deleted immediately upon reaching the healthy `Ready` state.
  * In Docker, configuration is supplied via environment variables (`docker-compose.yml` / `.env`).
* **Setup State:**
  * Dynamic: evaluated strictly from `users` table row count (`CountUsers(ctx) > 0`).
  * Once an administrator completes initial registration, the system is initialized. Any subsequent call to `POST /api/v1/auth/setup` returns HTTP 409 Conflict (`"system already initialized"`). Setup state is immutable and cannot be overwritten.

### 2. Migration Safety Guarantees

* **Forward-only and Embedded:** Schema migrations use `goose` and are embedded directly into the Go binary (`embed.FS`). No loose SQL migration files travel separately from the binary.
* **Transactional Execution:** Each migration file executes inside a database transaction (`-- +goose Up`). If a statement fails, changes in that file roll back.
* **Partial Migration Detection & Fail-Stop:** Goose records completed migrations in `goose_db_version`. If a migration fails partway, the startup sequence stops immediately and never proceeds to connection pool initialization or request serving.
* **Non-Destructive Recovery:** Failed migrations leave existing tables and records intact. When a bug in a migration is fixed or updated, the server seamlessly retries and applies the remaining migrations forward.
* **Existing Data Preservation:** Migrations adding new columns or constraints must provide non-null defaults, backfills, or nullable columns (e.g. `reading_progress.sync_sequence` sequence backfill in migration 00011) so existing user data is never truncated or dropped.

### 3. Restart Durability

* Restarts preserve all database tables, sequences, user accounts, credentials, library memberships, reading progress, and bookmarks.
* The Electron supervisor process and Docker restart policies (`restart: unless-stopped`) ensure automatic recovery on system reboot or unexpected process exit.
* A corrupted data directory or database connection failure triggers an explicit `Failed` state. Automatic re-initialization or deletion of existing data directories is strictly forbidden.

### 4. Backup & Disaster Recovery Runbook

#### Docker-Managed Hosting

* **Backup:**
  1. Dump the PostgreSQL database:
     ```bash
     docker exec alexandryn-postgres pg_dump -U admin -d alexandryn -F c -f /tmp/alexandryn.dump
     docker cp alexandryn-postgres:/tmp/alexandryn.dump ./alexandryn.dump
     ```
  2. Backup the cryptographic key:
     ```bash
     docker run --rm -v alexandryn_app-data:/data -v $(pwd):/backup alpine cp /data/source-credentials.key /backup/
     ```
* **Restore:**
  1. Restore the cryptographic key into `app-data`:
     ```bash
     docker run --rm -v alexandryn_app-data:/data -v $(pwd):/backup alpine cp /backup/source-credentials.key /data/
     ```
  2. Restore the database dump into a fresh PostgreSQL container:
     ```bash
     docker cp ./alexandryn.dump alexandryn-postgres:/tmp/alexandryn.dump
     docker exec alexandryn-postgres pg_restore -U admin -d alexandryn --clean --if-exists /tmp/alexandryn.dump
     ```
  3. Start the backend; `goose.Up` will verify and apply any new forward migrations automatically.

#### Desktop-Managed Hosting

* **Offline Backup:**
  1. Close Alexandryn.
  2. Copy the entire application data directory:
     * Linux: `~/.config/alexandryn/` (contains `pgdata/` and `source-credentials.key`).
     * Windows: `%APPDATA%\alexandryn\`
     * macOS: `~/Library/Application Support/alexandryn/`
* **Offline Restore:**
  1. Restore the copied directory to the same path before launching Alexandryn.
  2. Launch Alexandryn. The server will detect the existing `PG_VERSION` and `source-credentials.key`, run pending migrations if upgrading, and resume serving without data loss.

## Consequences

* **Positive:** Existing installations can be safely upgraded across releases without data loss or manual database interventions.
* **Positive:** Clear, documented backup and recovery paths exist for both Docker and Desktop deployment models.
* **Positive:** Cryptographic continuity is guaranteed across restarts while maintaining fail-closed safety on corrupt keys.
* **Neutral:** Backup operations require standard tooling (`docker exec` / filesystem copies); no heavyweight bespoke backup daemon is introduced.
