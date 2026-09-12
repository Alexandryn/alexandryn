import { expect, test } from '@playwright/test'

// End-to-end routing and shell composition walkthrough:
// one full path through the shell with mock data, and the keyboard-only
// "open a book" reference walkthrough, no pointer events.

test('routing and shell composition work end to end against mock data', async ({ page }) => {
  await page.goto('/library')

  await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
  await expect(page.getByRole('banner')).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible()
  await expect(page.getByRole('banner').getByRole('searchbox')).toBeVisible()
  await expect(page.getByRole('link', { name: /Middlemarch/ })).toBeVisible()

  await page.getByRole('link', { name: 'Discover' }).click()
  await expect(page).toHaveURL(/\/discover$/)
  await expect(page.getByRole('heading', { name: 'Discover', level: 1 })).toBeVisible()

  await page.goto('/no/such/page')
  await expect(page.getByRole('heading', { name: "This page doesn't exist" })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible()
})

test('keyboard-only: open a book from the library', async ({ page }) => {
  await page.goto('/library')
  await expect(page.getByRole('link', { name: /Middlemarch/ })).toBeVisible()

  // First Tab lands on the skip link; activating it moves focus into the
  // content region.
  await page.keyboard.press('Tab')
  await expect(page.getByRole('link', { name: 'Skip to content' })).toBeFocused()
  await page.keyboard.press('Enter')
  await expect(page.locator('#main')).toBeFocused()

  // Navigate to book link and open it
  await page.getByRole('link', { name: /Middlemarch/ }).focus()
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/\/book\/01JXXXXXXXXXXXXXXXXXXXXXXX$/)
  await expect(page.getByRole('heading', { name: 'Middlemarch', level: 1 })).toBeVisible()
  // Focus followed to the new screen (the shell's route-change handler).
  await expect(page.locator('#main')).toBeFocused()
})

test('reflows to the mobile tab bar below the breakpoint', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 })
  await page.goto('/library')
  await expect(page.getByRole('link', { name: 'Settings' })).toBeVisible() // sidebar-only

  await page.setViewportSize({ width: 480, height: 900 })
  await expect(page.getByRole('link', { name: 'More' })).toBeVisible() // tab-bar-only
  await expect(page.getByRole('link', { name: 'Settings' })).toBeHidden()
})
