import { execFileSync } from 'node:child_process'
import { existsSync, mkdtempSync, rmSync } from 'node:fs'
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
export interface LaunchHostOptions {
  extraArgs?: string[]
  env?: Record<string, string>
}

export async function launchHost(options: LaunchHostOptions | string[] = {}): Promise<ElectronApplication> {
  const extraArgs = Array.isArray(options) ? options : (options.extraArgs ?? [])
  const customEnv = Array.isArray(options) ? {} : (options.env ?? {})
  const userDataDir = mkdtempSync(join(tmpdir(), 'alexandryn-e2e-user-data-'))
  const ciFlags = process.env.CI ? ['--no-sandbox', '--disable-setuid-sandbox'] : []
  const testServerBinary = join(__dirname, '../test-helpers/test-server/test-server')

  if (!existsSync(testServerBinary)) {
    execFileSync('go', ['build', '-o', testServerBinary, './electron/test-helpers/test-server'], {
      cwd: join(__dirname, '../..'),
    })
  }


  const app = await _electron.launch({
    args: [MAIN, `--user-data-dir=${userDataDir}`, ...ciFlags, ...extraArgs],
    env: {
      ...process.env,
      NODE_ENV: 'test',
      ALEXANDRYN_SERVER_BINARY_PATH: testServerBinary,
      ...customEnv,
    },
  })

  const originalClose = app.close.bind(app)
  app.close = async () => {
    try {
      await originalClose()
    } finally {
      rmSync(userDataDir, { recursive: true, force: true })
    }
  }

  return app
}







