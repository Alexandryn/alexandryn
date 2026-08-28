import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// frontend-accessibility.md FR-4 / this spec's Test strategy: real-browser
// axe-core over the shell, at a desktop and a mobile viewport, distinct
// from Tier 0's lint-time jsx-a11y check.
//
// color-contrast is disabled here for the same reason src/test/axe.ts
// disables it in jsdom: token contrast is owned by
// `npm run tokens:check-contrast`, which already flags the one failing
// pair (text-3 against light surfaces, 2.5–2.9:1) as a documented
// exception pending a maintainer decision. Tier 5's F24 sets the final
// real-browser contrast policy; this tier does not pre-empt that call.
function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test('shell has no axe violations at desktop width (sidebar layout)', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await page.goto('/library')
  await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible()
  await expect(page.getByText('Invisible Cities')).toBeVisible()

  const results = await axe(page).analyze()
  expect(results.violations).toEqual([])
})

test('shell has no axe violations at mobile width (tab-bar layout)', async ({ page }) => {
  await page.setViewportSize({ width: 480, height: 900 })
  await page.goto('/library')
  await expect(page.getByRole('link', { name: 'More' })).toBeVisible()

  const results = await axe(page).analyze()
  expect(results.violations).toEqual([])
})

test('the not-found view has no axe violations', async ({ page }) => {
  await page.goto('/no/such/page')
  await expect(page.getByRole('heading', { name: "This page doesn't exist" })).toBeVisible()

  const results = await axe(page).analyze()
  expect(results.violations).toEqual([])
})

// frontend-accessibility.md FR-5: the per-primitive motion-reduce rules
// (Tier 2) actually compose at the app level. Hold the capability value
// so the host-only route's RequireCapability keeps rendering its Spinner,
// then check the spinning element under each media state.
async function spinnerAnimationCount(page: import('@playwright/test').Page): Promise<number> {
  await page.route('**/api/bootstrap', () => {
    /* never resolve — keep the loading state on screen */
  })
  await page.goto('/settings')
  const spinner = page.getByRole('status').locator('span[aria-hidden="true"]')
  await expect(spinner).toBeVisible()
  return spinner.evaluate((el) => el.getAnimations().length)
}

test('shell animations run by default but stop under prefers-reduced-motion', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'no-preference' })
  expect(await spinnerAnimationCount(page)).toBeGreaterThan(0)

  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.unroute('**/api/bootstrap')
  expect(await spinnerAnimationCount(page)).toBe(0)
})

test('the prefers-contrast adaptation is present in the served stylesheet', async ({ page }) => {
  // Playwright can't emulate prefers-contrast (maintainer decision G2), so
  // assert the mechanism ships: the served CSS carries the FR-5 override.
  await page.goto('/library')
  const hasContrastRule = await page.evaluate(() =>
    [...document.styleSheets].some((sheet) => {
      try {
        return [...sheet.cssRules].some(
          (rule) =>
            rule instanceof CSSMediaRule &&
            rule.conditionText.includes('prefers-contrast') &&
            rule.cssText.includes('--color-text-3'),
        )
      } catch {
        return false
      }
    }),
  )
  expect(hasContrastRule).toBe(true)
})
