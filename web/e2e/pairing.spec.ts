import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// Same exclusion as the other *.app.spec.ts axe helpers: color-contrast is
// owned by `npm run tokens:check-contrast`, not this sweep.
function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Phase 13: Network Access & Device Pairing E2E (T5.8)', () => {
  test('happy path: host opens modal, second context pairs and logs in', async ({ browser, baseURL }) => {
    const contextA = await browser.newContext({ baseURL })
    const contextB = await browser.newContext({ baseURL })

    const pageA = await contextA.newPage()
    const pageB = await contextB.newPage()

    try {
      // 1. Context A (Admin / Host) navigates to Network Settings
      await pageA.goto('/settings/network')
      await expect(pageA.getByRole('heading', { name: 'Network Access', level: 1 })).toBeVisible()

      // Axe audit on /settings/network before the modal opens (Phase 17
      // coverage sweep — this route otherwise has only this pairing-flow
      // spec exercising it, with no axe assertion).
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

      // Axe audit with the DevicePairingModal open (Phase 17 coverage
      // sweep — the modal opens in this spec but was never audited while
      // open).
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
    const context = await browser.newContext({ baseURL })
    const pageA = await context.newPage()
    const pageB = await context.newPage()

    try {
      // Context A opens modal
      await pageA.goto('/settings/network')
      await pageA.getByRole('button', { name: 'Pair a new device' }).click()
      await expect(pageA.getByRole('dialog', { name: 'Pair a Device' })).toBeVisible()
      await expect(pageA.getByTestId('pairing-code')).toBeVisible()

      // Host clicks Revoke
      await pageA.getByRole('button', { name: 'Revoke' }).click()
      await expect(pageA.getByRole('dialog', { name: 'Pair a Device' })).toBeHidden()

      // Context B tries to connect with code
      await pageB.goto('/connect?c=ABCD-EFGH')
      await pageB.getByRole('button', { name: 'Continue' }).click()

      // Code no longer verifies: error shown, focus returned
      const alert = pageB.getByRole('alert')
      await expect(alert).toBeVisible()
      await expect(alert).toHaveText(/pairing code not recognised/)
      await expect(pageB.getByLabel('Pairing Code')).toBeFocused()
    } finally {
      await context.close()
    }
  })
})
