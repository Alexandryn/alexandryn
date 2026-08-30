import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { _electron, type ElectronApplication } from '@playwright/test'

// Playwright transpiles specs as CJS (electron/ is not "type": "module"),
// so __dirname is available and import.meta is not.
const MAIN = join(__dirname, '../out/main/index.js')

/**
 * Launches the built desktop host from `electron/out` (electron-vite
 * output). Every `_electron` spec uses this so the launch args and env
 * are declared once. `ELECTRON_DISABLE_SANDBOX` is deliberately NOT set —
 * the sandbox is part of what these tests verify; CI runs under xvfb
 * with `--no-sandbox` only where the runner's own kernel forbids user
 * namespaces (handled by the CI job, not here).
 */
export function launchHost(extraArgs: string[] = []): Promise<ElectronApplication> {
  const userDataDir = mkdtempSync(join(tmpdir(), 'alexandryn-e2e-user-data-'))
  return _electron.launch({
    args: [`--user-data-dir=${userDataDir}`, MAIN, ...extraArgs],
    env: { ...process.env, NODE_ENV: 'test' },
  })
}



