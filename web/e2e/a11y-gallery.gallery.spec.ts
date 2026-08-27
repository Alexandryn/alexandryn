import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// frontend-accessibility.md FR-4 / Acceptance criterion 1: zero axe
// violations across every phase-04 primitive, scanned in a real browser.
// Distinct from Tier 0's lint-time eslint-plugin-jsx-a11y (write-time,
// structural) and from Tier 2's per-primitive jsdom axe (no layout
// engine) — this catches runtime-rendered issues those can't see.
//
// color-contrast stays disabled here, exactly as in a11y.app.spec.ts:
// contrast is owned by `npm run tokens:check-contrast`, which flags the
// one failing pair (text-3) as a documented exception pending a
// maintainer decision, and Tier 5 mitigates it under prefers-contrast
// rather than pre-empting that call.

const axe = (page: import('@playwright/test').Page) =>
  new AxeBuilder({ page }).disableRules(['color-contrast'])

test('every primitive renders with zero axe violations', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Accessibility gallery', level: 1 })).toBeVisible()

  const results = await axe(page).analyze()
  expect(results.violations).toEqual([])
})

test('the modal dialog has zero axe violations when open', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('button', { name: 'Delete collection…' }).click()
  await expect(page.getByRole('dialog', { name: 'Delete this collection?' })).toBeVisible()

  const results = await axe(page).analyze()
  expect(results.violations).toEqual([])
})
