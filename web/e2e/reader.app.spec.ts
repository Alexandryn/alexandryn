import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

// Tier 1 axe coverage for the Reader (audit 0017's largest stated coverage
// gap). src/mocks/handlers.ts now serves reader.fixtures.ts's minimal
// EPUB content by path suffix for any editionId, so no test-local mock is
// needed here.
function axe(page: Page) {
  // color-contrast excluded for the same reason a11y.app.spec.ts excludes
  // it (owned by npm run tokens:check-contrast); iframe excluded because
  // the reading-area iframe is sandbox="allow-same-origin" only, never
  // allow-scripts (FR-1) — axe cannot inject its scanner into it, same as
  // the jsdom-level axe check in Reader.test.tsx.
  return new AxeBuilder({ page }).disableRules(['color-contrast']).exclude('iframe')
}

test.describe('Reader: axe coverage', () => {
  test('EPUB reader has no axe violations', async ({ page }) => {
    await page.goto('/read/w1/e1')

    await expect(page.getByRole('link', { name: '← Library' })).toBeVisible()
    await expect(page.getByTitle(/reading area/i)).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })

  test('table of contents panel has no axe violations when open', async ({ page }) => {
    await page.goto('/read/w1/e1')
    await expect(page.getByTitle(/reading area/i)).toBeVisible()

    await page.getByRole('button', { name: 'Contents' }).click()
    await expect(page.getByRole('navigation', { name: 'Table of contents' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Chapter One' })).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
