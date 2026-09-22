# Alexandryn — desktop host

The Electron main process, preload script, and the disk-loaded
loading/error boot asset.
Package name `@alexandryn/desktop`; the directory is `electron/`.
It does **not** bundle the web UI — the Electron window loads
the real UI from the Go server's own loopback URL at runtime, the same
as a LAN browser would.

## Build-order dependency

The desktop host resolves the Go server binary from
`<repo-root>/bin/alexandryn-server` in development. Build it first:

```
go build -o bin/alexandryn-server ./cmd/server
```

then run the desktop host. `npm run dev` does not build the Go binary —
that step is yours (in CI the `backend` job produces it and the
`desktop` job downloads it).

## Bundled PostgreSQL (ADR 0007)

A packaged build bundles real `postgres`/`initdb` binaries so it needs no
system-installed PostgreSQL. Fetch them once before packaging (Linux and
Windows only — macOS has no `cmd/server` spawn implementation yet, see
`spawn_darwin.go`):

```
npm run -w @alexandryn/desktop postgres:fetch
```

This downloads a pinned, checksum-verified PostgreSQL 16 build (matching the
project's Docker target) from `io.zonky.test.postgres` into
`electron/resources/postgres` and extracts it with the system `tar` — no new
npm dependency. It is a no-op if that directory already exists; delete it to
re-fetch. `electron-builder.yml`'s `extraResources` (Linux/Windows sections)
bundles the result; `postgresBinaries.ts` finds it there in a packaged build,
or under `electron/resources/postgres/bin` in a dev build run after fetching.

## Commands

Run from the repo root (npm workspace).

| Command                                         | Does                                                                                      |
| ----------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `npm run -w @alexandryn/desktop dev`            | electron-vite dev server + Electron, main-process HMR                                     |
| `npm run -w @alexandryn/desktop build`          | electron-vite build → `electron/out/` (main / preload / boot renderer)                    |
| `npm run -w @alexandryn/desktop typecheck`      | `tsc --noEmit`                                                                            |
| `npm run -w @alexandryn/desktop lint`           | ESLint                                                                                    |
| `npm run -w @alexandryn/desktop format:check`   | Prettier (shared root `.prettierrc.json`)                                                 |
| `npm run -w @alexandryn/desktop test`           | Vitest — unit tests (`src/**`)                                                            |
| `npm run -w @alexandryn/desktop test:e2e`       | `@playwright/test` `_electron` lifecycle (needs a display — `xvfb-run -a` on headless CI) |
| `npm run -w @alexandryn/desktop postgres:fetch` | Stages the bundled PostgreSQL for packaging (below)                                       |

All of the above run in CI's `desktop` job (`.github/workflows/ci.yml`),
which `needs: [frontend, backend]`.
