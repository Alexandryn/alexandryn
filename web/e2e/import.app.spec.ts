import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Import screen: axe coverage', () => {
  test('/import has no axe violations', async ({ page }) => {
    await page.goto('/import')
    await expect(page.getByRole('heading', { name: 'Import review', level: 1 })).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
