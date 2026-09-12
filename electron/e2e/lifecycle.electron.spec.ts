import { expect, test, type ElectronApplication } from '@playwright/test'
import { launchHost } from './launch'

// Full lifecycle walkthrough:
// 1. Cold start failure shows loading → error state on boot asset with accessible Retry button.
// 2. Ready server transitions window to real UI.
// 3. Complete keyboard-only "open a book" slice inside real Electron window.

let app: ElectronApplication

test.afterEach(async () => {
  if (app) {
    await app.close()
  }
})

test('lifecycle: cold start failure displays error state on boot asset with retry button', async () => {
  // Launch with non-existent binary to force immediate startup failure
  app = await launchHost({
    env: { ALEXANDRYN_SERVER_BINARY_PATH: '/no/such/binary' },
  })
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  await expect(window.locator('#error-state')).toBeVisible({ timeout: 15_000 })
  await expect(window.locator('#loading-state')).toBeHidden()
  await expect(window.locator('#error-state .state-title')).toHaveText('Unable to start Alexandryn')
  await expect(window.locator('#retry-btn')).toBeVisible()
})

test('lifecycle: manual retry on error screen reloads boot asset and re-attempts startup', async () => {
  app = await launchHost({
    env: { ALEXANDRYN_SERVER_BINARY_PATH: '/no/such/binary' },
  })
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  const retryBtn = window.locator('#retry-btn')
  await expect(retryBtn).toBeVisible({ timeout: 15_000 })

  // Click Retry to trigger reload and restart lifecycle
  await retryBtn.click()

  // Verifies that after retry, server startup is re-attempted and error state is re-displayed
  await expect(window.locator('#error-state')).toBeVisible({ timeout: 15_000 })
  await expect(window.locator('#error-state .state-title')).toHaveText('Unable to start Alexandryn')
})








test('lifecycle: ready server transitions to real UI and serves application', async () => {

  app = await launchHost()
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  // Real UI rendered with Library navigation and content
  await expect(window.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible({
    timeout: 15_000,
  })
})


test('lifecycle: keyboard-only "open a book" slice completes inside Electron', async () => {
  app = await launchHost()
  const window = await app.firstWindow()
  await window.waitForLoadState('domcontentloaded')

  await expect(window.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible({
    timeout: 15_000,
  })
  await expect(window.getByText('Invisible Cities')).toBeVisible()

  // First Tab lands on the skip link; activating it moves focus into the content region.
  await window.keyboard.press('Tab')
  await expect(window.getByRole('link', { name: 'Skip to content' })).toBeFocused()
  await window.keyboard.press('Enter')
  await expect(window.locator('#main')).toBeFocused()

  // Next Tab reaches the book link.
  await window.keyboard.press('Tab')
  await expect(window.getByRole('link', { name: 'Invisible Cities' })).toBeFocused()

  // Activating the book link navigates to the book detail view.
  await window.keyboard.press('Enter')
  await expect(window).toHaveURL(/\/book\/ol-1$/)
  await expect(window.getByRole('heading', { name: 'Book', level: 1 })).toBeVisible()
  await expect(window.locator('#main')).toBeFocused()
})
