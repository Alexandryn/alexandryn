import { expect, test, type ElectronApplication } from '@playwright/test'
import { launchHost } from './launch'

// Hostile walkthrough:
// 1. window.open('https://evil.example') is intercepted and denied.
// 2. Navigation attempts to external origins, file://, and javascript: schemes are blocked.
// 3. Renderer cannot access undeclared IPC channels or raw ipcRenderer.

let app: ElectronApplication

test.beforeEach(async () => {
  app = await launchHost({ env: { ALEXANDRYN_SKIP_SERVER_LIFECYCLE: '1' } })
})


test.afterEach(async () => {
  if (app) {
    await app.close()
  }
})

test('hostile: window.open to external origin is intercepted and denied', async () => {
  const page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

  const popupCreated = await page.evaluate(() => {
    const popup = window.open('https://evil.example.com/malicious', '_blank')
    return popup !== null
  })

  // setWindowOpenHandler returns { action: 'deny' }, so window.open returns null
  expect(popupCreated).toBe(false)
  expect(app.windows()).toHaveLength(1)
})

test('hostile: will-navigate intercepts and blocks attempts to navigate away from allowed origin', async () => {
  const page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

  const initialUrl = page.url()

  // Attempt navigation to external HTTPS origin
  await page.evaluate(() => {
    window.location.href = 'https://evil.example.com/phishing'
  })

  // Short pause to allow any potential navigation to take effect
  await page.waitForTimeout(500)
  expect(page.url()).toBe(initialUrl)

  // Attempt navigation to file:// origin
  await page.evaluate(() => {
    window.location.href = 'file:///etc/passwd'
  })
  await page.waitForTimeout(500)
  expect(page.url()).toBe(initialUrl)

  // Attempt navigation to javascript: scheme
  await page.evaluate(() => {
    window.location.href = 'javascript:void(0)'
  })
  await page.waitForTimeout(500)
  expect(page.url()).toBe(initialUrl)
})

test('hostile: raw ipcRenderer, process, and node globals are unexposed', async () => {
  const page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

  const isolation = await page.evaluate(() => {
    const win = window as unknown as Record<string, unknown>
    return {
      hasIpcRenderer: typeof win.ipcRenderer !== 'undefined',
      hasRequire: typeof win.require !== 'undefined',
      hasProcess: typeof win.process !== 'undefined',
      hasBuffer: typeof win.Buffer !== 'undefined',
      hasGlobal: typeof win.global !== 'undefined',
    }
  })

  expect(isolation.hasIpcRenderer).toBe(false)
  expect(isolation.hasRequire).toBe(false)
  expect(isolation.hasProcess).toBe(false)
  expect(isolation.hasBuffer).toBe(false)
  expect(isolation.hasGlobal).toBe(false)
})

test('hostile: preload methods with unexpected/malformed args are rejected by Zod schema', async () => {
  const page = await app.firstWindow()
  await page.waitForLoadState('domcontentloaded')

  // Calling system.getAppVersion with an unexpected argument must reject pre-handler with "Invalid arguments"
  const getAppVersionError = await page.evaluate(async () => {
    try {
      const alex = (window as unknown as { alexandryn: { system: { getAppVersion: (arg: unknown) => Promise<string> } } }).alexandryn
      await alex.system.getAppVersion('unexpected-payload')
      return null
    } catch (err) {
      return (err as Error).message
    }
  })
  expect(getAppVersionError).toMatch(/Invalid arguments/)

  // Calling source.pickLocalFolder with an unexpected extra payload must reject pre-handler with "Invalid arguments"
  const pickFolderError = await page.evaluate(async () => {
    try {
      const alex = (window as unknown as { alexandryn: { source: { pickLocalFolder: (arg: unknown) => Promise<unknown> } } }).alexandryn
      await alex.source.pickLocalFolder({ maliciousField: true })
      return null
    } catch (err) {
      return (err as Error).message
    }
  })
  expect(pickFolderError).toMatch(/Invalid arguments/)
})


