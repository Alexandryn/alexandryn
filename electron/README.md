# Alexandryn — desktop host

The Electron main process, preload script, and the disk-loaded
loading/error boot asset (`architecture-desktop-host.md`, phase 05).
Package name `@alexandryn/desktop`; the directory is `electron/` per
ADR 0008. It does **not** bundle the web UI — the Electron window loads
the real UI from the Go server's own loopback URL at runtime, the same
as a LAN browser would.

## Build-order dependency

`desktop-host-process-model.md` FR-1 resolves the Go server binary from
`<repo-root>/bin/alexandryn-server` in development. Build it first:

```
go build -o bin/alexandryn-server ./cmd/server
```

then run the desktop host. `npm run dev` does not build the Go binary —
that step is yours (in CI the `backend` job produces it and the
`desktop` job downloads it).

## Commands

Run from the repo root (npm workspace).

| Command                                       | Does                                                                                      |
| --------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `npm run -w @alexandryn/desktop dev`          | electron-vite dev server + Electron, main-process HMR                                     |
| `npm run -w @alexandryn/desktop build`        | electron-vite build → `electron/out/` (main / preload / boot renderer)                    |
| `npm run -w @alexandryn/desktop typecheck`    | `tsc --noEmit`                                                                            |
| `npm run -w @alexandryn/desktop lint`         | ESLint                                                                                    |
| `npm run -w @alexandryn/desktop format:check` | Prettier (shared root `.prettierrc.json`)                                                 |
| `npm run -w @alexandryn/desktop test`         | Vitest — unit tests (`src/**`)                                                            |
| `npm run -w @alexandryn/desktop test:e2e`     | `@playwright/test` `_electron` lifecycle (needs a display — `xvfb-run -a` on headless CI) |

All of the above run in CI's `desktop` job (`.github/workflows/ci.yml`),
which `needs: [frontend, backend]`.
