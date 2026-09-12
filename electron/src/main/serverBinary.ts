import { join } from 'node:path'
import { app } from 'electron'

const BINARY_STEM = 'alexandryn-server'

/** `alexandryn-server.exe` on Windows, `alexandryn-server` everywhere else. */
export function platformBinaryName(): string {
  return process.platform === 'win32' ? `${BINARY_STEM}.exe` : BINARY_STEM
}

/**
 * The single source of truth for where the Go server binary lives.
 * Never re-derive this path anywhere else — the spawn call is its one caller.
 *
 * - **Development** (`!app.isPackaged`): `<repo-root>/bin/<name>`.
 *   The Go module is at the repo root and the electron package one
 *   level down, so the repo root is the parent of `app.getAppPath()`.
 *   `go build -o bin/alexandryn-server ./cmd/server` must run before
 *   `npm run dev` — a build-order dependency, documented in
 *   `electron/README.md`.
 * - **Packaged**: `process.resourcesPath/server/<name>` — Electron's own
 *   resources convention, independent of the source-tree layout.
 */
export function resolveServerBinaryPath(): string {
  if (process.env.ALEXANDRYN_SERVER_BINARY_PATH) {
    return process.env.ALEXANDRYN_SERVER_BINARY_PATH
  }

  const name = platformBinaryName()


  if (app.isPackaged) {
    const { resourcesPath } = process as NodeJS.Process & { resourcesPath?: string }
    if (!resourcesPath) {
      throw new Error(
        'resolveServerBinaryPath: app.isPackaged but process.resourcesPath is unset — cannot locate the bundled server binary',
      )
    }
    return join(resourcesPath, 'server', name)
  }

  return join(app.getAppPath(), '..', 'bin', name)
}
