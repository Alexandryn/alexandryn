import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('More screen: axe coverage', () => {
  test('/more has no axe violations', async ({ page }) => {
    await page.goto('/more')
    await expect(page.getByRole('heading', { name: 'More', level: 1 })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Activity', exact: true })).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
