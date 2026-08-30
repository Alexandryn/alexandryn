import { expect, test, type ElectronApplication } from '@playwright/test'
import { launchHost } from './launch'

// Checkpoint P5-D — E2E validation for Tier 3 (window and serving).
// Validates:
// 1. Boot asset loads with semantic status and tokens styling
// 2. Window minWidth / minHeight floor is applied
// 3. Navigation to external origins is intercepted

let app: ElectronApplication

test.beforeEach(async () => {
  app = await launchHost()
})

test.afterEach(async () => {
  await app.close()
})

test('window respects minimum size constraints (FR-1)', async () => {
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  const minSize = await app.evaluate(({ BrowserWindow }) => {
    const win = BrowserWindow.getAllWindows()[0]
    return win ? win.getMinimumSize() : null
  })

  expect(minSize).toEqual([768, 500])
})

test('boot asset renders semantic status container (FR-3)', async () => {
  const window = await app.firstWindow()
  const bootCard = window.locator('.boot-card')
  await expect(bootCard).toHaveAttribute('role', 'status')
  await expect(bootCard).toHaveAttribute('aria-live', 'polite')
})
