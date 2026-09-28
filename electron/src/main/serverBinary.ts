import { chmodSync, existsSync, statSync } from 'node:fs'
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

/**
 * Checks if the server binary is installed and present on disk.
 */
export function isServerBinaryInstalled(
  binaryPathResolver: () => string = resolveServerBinaryPath,
): boolean {
  try {
    const path = binaryPathResolver()
    return existsSync(path)
  } catch {
    return false
  }
}

/**
 * Ensures the binary has executable permissions on POSIX systems (0o755).
 * Safe no-op on Windows or read-only filesystems.
 */
export function ensureBinaryExecutable(path: string): void {
  if (process.platform === 'win32') return
  try {
    const stat = statSync(path)
    if ((stat.mode & 0o111) === 0) {
      chmodSync(path, stat.mode | 0o755)
    }
  } catch {
    // Ignore error if file is on a read-only filesystem (e.g. AppImage squashfs)
  }
}
