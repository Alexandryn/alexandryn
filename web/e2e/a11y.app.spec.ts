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
  await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()

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

// The reduced-motion composed check (FR-5) lives in the a11y gallery
// project (e2e/a11y-gallery.gallery.spec.ts) — that harness renders
// Spinner / Skeleton / ProgressBar permanently and has no service worker,
// so emulateMedia and getAnimations() are deterministic there. Holding
// the app's loading state instead would race MSW's ~50ms mock (and
// page.route can't intercept a service-worker-answered request).

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
