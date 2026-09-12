import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type ElectronApplication } from '@playwright/test'
import { launchHost } from './launch'

// E2E validation for IPC surface.
// Verifies:
// 1. window.alexandryn namespace structure exists in renderer
// 2. window.alexandryn.system.getAppVersion() returns package.json version
// 3. window.alexandryn.source.pickLocalFolder() is callable
// 4. Raw ipcRenderer is not exposed and undeclared channels do not exist

let app: ElectronApplication

test.beforeEach(async () => {
  app = await launchHost({ env: { ALEXANDRYN_SKIP_SERVER_LIFECYCLE: '1' } })
})


test.afterEach(async () => {
  await app.close()
})

test('window.alexandryn bridge surface is exposed and sandboxed', async () => {
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  const evaluation = await window.evaluate(() => {
    const win = window as unknown as Record<string, unknown>
    const alex = (window as unknown as { alexandryn?: Record<string, unknown> }).alexandryn
    return {
      hasBridge: typeof alex !== 'undefined',
      namespaces: alex ? Object.keys(alex) : [],
      systemMethods: alex ? Object.keys((alex.system as Record<string, unknown>) ?? {}) : [],
      sourceMethods: alex ? Object.keys((alex.source as Record<string, unknown>) ?? {}) : [],
      isPickLocalFolderFn: typeof (alex?.source as Record<string, unknown> | undefined)?.pickLocalFolder === 'function',
      hasIpcRenderer: typeof win.ipcRenderer !== 'undefined',
      hasRequire: typeof win.require !== 'undefined',
      hasProcess: typeof win.process !== 'undefined',
      hasUndeclaredChannel: typeof (alex as Record<string, unknown> | undefined)?.readFile !== 'undefined',
    }
  })

  expect(evaluation.hasBridge).toBe(true)
  expect(evaluation.namespaces).toEqual(['system', 'source'])
  expect(evaluation.systemMethods).toEqual(['getAppVersion', 'retryStartup'])
  expect(evaluation.sourceMethods).toEqual(['pickLocalFolder'])

  expect(evaluation.isPickLocalFolderFn).toBe(true)
  expect(evaluation.hasIpcRenderer).toBe(false)
  expect(evaluation.hasRequire).toBe(false)
  expect(evaluation.hasProcess).toBe(false)
  expect(evaluation.hasUndeclaredChannel).toBe(false)
})

test('system.getAppVersion() returns correct package.json version through real IPC boundary', async () => {
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  const pkgJson = JSON.parse(readFileSync(join(__dirname, '../package.json'), 'utf8')) as { version: string }

  const version = await window.evaluate(async () => {
    const alex = (window as unknown as { alexandryn: { system: { getAppVersion: () => Promise<string> } } }).alexandryn
    return alex.system.getAppVersion()
  })

  expect(version).toBe(pkgJson.version)
})
