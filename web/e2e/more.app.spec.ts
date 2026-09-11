import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('More screen: axe coverage', () => {
  test('/more has no axe violations', async ({ page }) => {
    await page.goto('/more')
    await expect(page.getByRole('heading', { name: 'More', level: 1 })).toBeVisible()
    // Scoped to the "More sections" NavList, not a bare name match: at
    // desktop width the Sidebar also renders a plain "Activity" link (no
    // description), which `{ name: 'Activity', exact: true }` matched
    // instead of this screen's own content — passing desktop by
    // accident while genuinely failing at mobile width, where
    // MobileTabBar has no such link. NavList's Activity item's real
    // accessible name is "Activity Imports and background jobs." (its
    // description is part of the link, by design), so this matches on
    // the substring within the correct region regardless of viewport.
    await expect(
      page.getByRole('navigation', { name: 'More sections' }).getByRole('link', { name: /Activity/ }),
    ).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
