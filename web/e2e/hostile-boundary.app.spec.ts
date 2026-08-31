import { expect, test } from '@playwright/test'

test.describe('Phase 06: Hostile Boundary & Malformed Inputs (L22)', () => {
  test('malformed cursor query parameter in library screen', async ({ page }) => {
    await page.goto('/library?cursor=bad_cursor_payload_12345%21%40%23')

    // Shell remains intact and renders
    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
  })

  test('invalid filter or sort parameter defaults safely', async ({ page }) => {
    await page.goto('/library?filter=nonexistent_filter&sort=corrupted_sort')

    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
    await expect(page.getByRole('link', { name: /Middlemarch/ })).toBeVisible()
  })

  test('non-existent work ID renders dedicated 404 without retry button', async ({
    page,
  }) => {
    await page.goto('/book/non-existent-work-id-999999')

    await expect(page.getByText("This book isn't in your library.")).toBeVisible()
    await expect(page.getByRole('button', { name: 'Back to Library' })).toBeVisible()
    // Constitution §11: 404 error states must not show a retry button
    await expect(page.getByRole('button', { name: /try again/i })).toBeHidden()
  })

  test('non-existent collection ID renders dedicated 404 without retry button', async ({
    page,
  }) => {
    await page.goto('/collections/non-existent-collection-id-999999')

    await expect(page.getByText("This collection doesn't exist.")).toBeVisible()
    await expect(page.getByRole('button', { name: 'Back to Collections' })).toBeVisible()
    // Constitution §11: 404 error states must not show a retry button
    await expect(page.getByRole('button', { name: /try again/i })).toBeHidden()
  })

  test('boundary string lengths and special characters in URL do not break routing', async ({
    page,
  }) => {
    const longPayload = 'A'.repeat(500)
    await page.goto(`/library?q=${encodeURIComponent(longPayload)}`)

    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
    await expect(page.getByRole('navigation', { name: 'Primary' })).toBeVisible()
  })
})
