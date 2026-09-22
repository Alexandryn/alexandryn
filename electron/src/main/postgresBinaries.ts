import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { app } from 'electron'

// ADR 0007: a bundled, managed PostgreSQL, invisible to the user. The
// binaries themselves are staged by scripts/fetch-postgres-binaries.mjs
// (electron-builder's extraResources bundles them into a packaged build;
// see electron-builder.yml). This never ships on macOS — the darwin build
// of cmd/server (spawn_darwin.go) refuses to spawn one at all — only Linux
// and Windows.

/**
 * Where the bundled `postgres`/`initdb` binaries live: dev reads
 * `<electron-package>/resources/postgres/bin`, a packaged build reads
 * `process.resourcesPath/postgres/bin`. Returns undefined when nothing was
 * bundled there (every dev environment, and any platform this build didn't
 * stage binaries for) — that is not an error, it means the Go server falls
 * back to whatever postgres/initdb the system already has on PATH, which is
 * how development and CI have always run this path.
 */
export function resolvePostgresBinDir(): string | undefined {
  const dir = app.isPackaged
    ? (() => {
        const { resourcesPath } = process as NodeJS.Process & { resourcesPath?: string }
        return resourcesPath ? join(resourcesPath, 'postgres', 'bin') : undefined
      })()
    : join(app.getAppPath(), 'resources', 'postgres', 'bin')

  return dir !== undefined && existsSync(dir) ? dir : undefined
}

/** Puts `binDir` first on `PATH`, so it is found before any system install. */
export function pathWithPostgresBinFirst(
  binDir: string,
  existingPath: string | undefined,
  delimiter: string,
): string {
  return existingPath ? `${binDir}${delimiter}${existingPath}` : binDir
}
