import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Activity screen: axe coverage', () => {
  test('/activity has no axe violations', async ({ page }) => {
    // handlers.ts has no shared mock for GET /api/v1/activity/events (only
    // per-test handlers install one — see Activity's own unit tests), so
    // against the e2e harness's dev server this request 404s and the
    // screen renders its error state. That is still a real, stable DOM
    // state worth an axe pass over.
    await page.goto('/activity')
    await expect(page.getByRole('heading', { name: 'Activity', level: 1 })).toBeVisible()
    await expect(page.getByText('Failed to load activity feed.')).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
