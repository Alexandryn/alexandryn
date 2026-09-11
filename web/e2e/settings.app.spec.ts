import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Settings screens: axe coverage', () => {
  test('/settings index has no axe violations', async ({ page }) => {
    await page.goto('/settings')
    await expect(page.getByRole('heading', { name: 'Settings', level: 1 })).toBeVisible()
    await expect(page.getByRole('link', { name: /Network/ })).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })

  test('/settings/devices has no axe violations', async ({ page }) => {
    // handlers.ts has no shared mock for GET /api/v1/devices (only
    // per-test handlers install one — see DevicesSettings' own unit
    // tests), so against the e2e harness's dev server this request 404s
    // and the screen renders its error state. Still a real, stable DOM
    // state worth an axe pass over.
    await page.goto('/settings/devices')
    await expect(page.getByRole('heading', { name: 'Devices' })).toBeVisible()
    await expect(page.getByText('Failed to load devices list.')).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
