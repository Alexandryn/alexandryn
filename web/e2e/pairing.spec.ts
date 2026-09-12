import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// Same exclusion as the other *.app.spec.ts axe helpers: color-contrast is
// owned by `npm run tokens:check-contrast`, not this sweep.
function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Network Access & Device Pairing E2E', () => {
  test('happy path: host opens modal, second context pairs and logs in', async ({ browser, baseURL }) => {
    const contextA = await browser.newContext({ baseURL })
    const contextB = await browser.newContext({ baseURL })

    const pageA = await contextA.newPage()
    const pageB = await contextB.newPage()

    try {
      // 1. Context A (Admin / Host) navigates to Network Settings
      await pageA.goto('/settings/network')
      await expect(pageA.getByRole('heading', { name: 'Network Access', level: 1 })).toBeVisible()

      // Axe audit on /settings/network before the modal opens
      const networkPageAxe = await axe(pageA).analyze()
      expect(networkPageAxe.violations).toEqual([])

      // Context A opens DevicePairingModal
      await pageA.getByRole('button', { name: 'Pair a new device' }).click()
      await expect(pageA.getByRole('dialog', { name: 'Pair a Device' })).toBeVisible()

      // Read code from DOM (data-testid="pairing-code")
      const pairingCodeEl = pageA.getByTestId('pairing-code')
      await expect(pairingCodeEl).toBeVisible()
      const code = (await pairingCodeEl.textContent())?.trim()
      expect(code).toBeTruthy()
      expect(code).toBe('ABCD-EFGH')

      // Axe audit with the DevicePairingModal open
      const modalAxe = await axe(pageA).analyze()
      expect(modalAxe.violations).toEqual([])

      // 2. Context B (Second Browser / Reader) navigates to /connect?c=<code>
      await pageB.goto(`/connect?c=${code}`)

      // Verify ?c= is stripped from URL immediately
      await expect(pageB).not.toHaveURL(/\?c=/)
      const searchB = await pageB.evaluate(() => window.location.search)
      expect(searchB).toBe('')

      // Verify input prefilled with formatted code
      const codeInputB = pageB.getByLabel('Pairing Code')
      await expect(codeInputB).toHaveValue('ABCD-EFGH')

      // Fill device name (optional) and submit
      const labelInputB = pageB.getByLabel('Name this device (optional)')
      await labelInputB.fill('Living Room Tablet')
      await pageB.getByRole('button', { name: 'Continue' }).click()

      // 3. Context B redirected to /login with grant in router state (not URL)
      await expect(pageB).toHaveURL(/\/login$/)
      const loginSearchB = await pageB.evaluate(() => window.location.search)
      expect(loginSearchB).toBe('')

      // 4. Context B logs in
      await pageB.getByLabel('Email or Username').fill('reader')
      await pageB.getByLabel('Password').fill('reader-pass')
      await pageB.getByRole('button', { name: 'Sign In' }).click()

      // Context B lands in the library with valid session
      await expect(pageB).toHaveURL(/\/library$/)
      await expect(pageB.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()

      // 5. Context A clicks "Done" — modal closes, session cleaned up
      await pageA.getByRole('button', { name: 'Done' }).click()
      await expect(pageA.getByRole('dialog', { name: 'Pair a Device' })).toBeHidden()
    } finally {
      await contextA.close()
      await contextB.close()
    }
  })

  test('unhappy path: invalid code shows error and returns focus', async ({ page }) => {
    await page.goto('/connect')

    const codeInput = page.getByLabel('Pairing Code')
    await codeInput.fill('0000-0000')
    await page.getByRole('button', { name: 'Continue' }).click()

    // Generic error shown
    const alert = page.getByRole('alert')
    await expect(alert).toBeVisible()
    await expect(alert).toHaveText(/pairing code not recognised/)

    // Focus returns to input
    await expect(codeInput).toBeFocused()
  })

  test('unhappy path: host revoke invalidates code for second context', async ({ browser, baseURL }) => {
    // One page per context, never two pages sharing one. Firefox,
    // driven by Playwright, never claims a second page
    // opened in a context whose service worker is already active: the
    // registration reports `activated`, but that page's
    // navigator.serviceWorker.controller stays null permanently, so MSW
    // never intercepts, its worker.start() never resolves, and main.tsx
    // — which calls render() in .finally() — leaves a blank page.
    // Isolated against real Firefox: it reproduces with two bare pages
    // and no pairing involved, survives a reload and a 40s wait, and
    // does not happen with one page navigating twice or with one page
    // per context (which is why the happy-path test above is fine).
    const contextA = await browser.newContext({ baseURL })
    const contextB = await browser.newContext({ baseURL })
    const pageA = await contextA.newPage()

    try {
      // Context A opens modal
      await pageA.goto('/settings/network')
      await pageA.getByRole('button', { name: 'Pair a new device' }).click()
      await expect(pageA.getByRole('dialog', { name: 'Pair a Device' })).toBeVisible()
      await expect(pageA.getByTestId('pairing-code')).toBeVisible()

      // Host clicks Revoke
      await pageA.getByRole('button', { name: 'Revoke' }).click()
      await expect(pageA.getByRole('dialog', { name: 'Pair a Device' })).toBeHidden()

      // The mock backend's revoke state (alexandryn_mock_revoked, which
      // src/mocks/handlers.ts's pair/verify handler reads, and its
      // DELETE pair/:id handler writes) lives in localStorage, so it is
      // per-context by construction. Assert the host's revoke actually
      // set it — that is the half of the chain this test owns — then
      // carry the mock backend's state into the second device's context,
      // which a shared real backend would have done on its own.
      await expect
        .poll(() => pageA.evaluate(() => localStorage.getItem('alexandryn_mock_revoked')))
        .toBe('true')
      await contextB.addInitScript(() => {
        window.localStorage.setItem('alexandryn_mock_revoked', 'true')
      })

      // Context B tries to connect with code. Waiting for the code to
      // actually be prefilled (matching the happy-path test's own care)
      // instead of clicking immediately, so a slow-hydrating page fails
      // with a clear assertion here rather than a bare button-not-found.
      const pageB = await contextB.newPage()
      await pageB.goto('/connect?c=ABCD-EFGH')
      await expect(pageB.getByLabel('Pairing Code')).toHaveValue('ABCD-EFGH')
      await pageB.getByRole('button', { name: 'Continue' }).click()

      // Code no longer verifies: error shown, focus returned
      const alert = pageB.getByRole('alert')
      await expect(alert).toBeVisible()
      await expect(alert).toHaveText(/pairing code not recognised/)
      await expect(pageB.getByLabel('Pairing Code')).toBeFocused()
    } finally {
      await contextA.close()
      await contextB.close()
    }
  })
})
