import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

// Tier 1 axe coverage for the public, unauthenticated auth screens
// (routes.tsx renders these outside RequireAuth, so no MSW auth-state
// setup is needed).
function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Public auth screens: axe coverage', () => {
  test('/login has no axe violations', async ({ page }) => {
    await page.goto('/login')
    await expect(page.getByRole('heading', { name: 'Sign in', level: 1 })).toBeVisible()
    await expect(page.getByLabel('Email or Username')).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })

  test('/setup has no axe violations', async ({ page }) => {
    await page.goto('/setup')
    await expect(
      page.getByRole('heading', { name: 'Welcome to Alexandryn', level: 1 }),
    ).toBeVisible()
    await expect(page.getByLabel('Admin Username')).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })

  test('/forgot-password has no axe violations', async ({ page }) => {
    await page.goto('/forgot-password')
    await expect(
      page.getByRole('heading', { name: 'Reset your password', level: 1 }),
    ).toBeVisible()
    await expect(page.getByLabel('Email')).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })

  test('/reset-password has no axe violations', async ({ page }) => {
    // With no ?token=, the screen renders an error/redirect alert instead
    // of the form (ResetPasswordScreen.tsx) — a token exercises the real
    // form state, which is the state worth auditing.
    await page.goto('/reset-password?token=e2e-fixture-token')
    await expect(
      page.getByRole('heading', { name: 'Set a new password', level: 1 }),
    ).toBeVisible()
    // exact: true — 'New password' otherwise partial-matches 'Confirm new password' too
    await expect(page.getByLabel('New password', { exact: true })).toBeVisible()

    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })
})
