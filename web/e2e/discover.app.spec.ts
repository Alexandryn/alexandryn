import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Discover E2E Walkthrough', () => {
  test('discover home idle state, search, and results grid with axe audit', async ({
    page,
  }) => {
    await page.goto('/discover')

    // Discover header & search box
    await expect(page.getByRole('heading', { name: 'Discover', level: 1 })).toBeVisible()
    const searchInput = page.getByLabel('Search Open Library')
    await expect(searchInput).toBeVisible()

    // Idle state initial text
    await expect(page.getByText(/Search millions of books by title/)).toBeVisible()

    // Axe audit on idle state
    const idleAxe = await axe(page).analyze()
    expect(idleAxe.violations).toEqual([])

    // Type query and verify URL sync with debounce
    await searchInput.fill('Middlemarch')
    await expect(page).toHaveURL(/q=Middlemarch/)

    // Results grid renders
    await expect(page.getByText('Middlemarch').first()).toBeVisible()
    await expect(page.getByText('George Eliot').first()).toBeVisible()

    // Screen reader announcement region exists and is polite
    const liveRegion = page.locator('[aria-live="polite"]')
    await expect(liveRegion).toBeAttached()

    // Axe audit on search results
    const resultsAxe = await axe(page).analyze()
    expect(resultsAxe.violations).toEqual([])
  })

  test('navigate from search result to work detail and view metadata', async ({
    page,
  }) => {
    await page.goto('/discover?q=Middlemarch')

    // Click on the result card link
    const resultLink = page.getByRole('link', { name: /Middlemarch/ }).first()
    await expect(resultLink).toBeVisible()
    await resultLink.click()

    // Work detail route
    await expect(page).toHaveURL(/\/discover\/works\/OL82563W/)

    // Heading, author, and description
    await expect(page.getByRole('heading', { name: 'Middlemarch', level: 1 })).toBeVisible()
    await expect(page.getByText('A Study of Provincial Life', { exact: true })).toBeVisible()
    await expect(page.getByText('By George Eliot')).toBeVisible()
    await expect(page.getByText(/novel by Mary Anne Evans/)).toBeVisible()

    // Subjects chips
    await expect(page.getByText('Provincial life', { exact: true })).toBeVisible()
    await expect(page.getByText('Fiction', { exact: true })).toBeVisible()

    // Editions list
    await expect(page.getByText(/Editions/)).toBeVisible()
    await expect(page.getByText('Penguin Classics')).toBeVisible()

    // Back to Discover navigation
    const backLink = page.getByRole('link', { name: '← Back to Discover' })
    await expect(backLink).toBeVisible()

    // Axe audit on Work Detail screen
    const detailAxe = await axe(page).analyze()
    expect(detailAxe.violations).toEqual([])

    // Click back to discover
    await backLink.click()
    await expect(page).toHaveURL(/\/discover/)
  })

  test('degraded 503 and 404 error states on Discover', async ({ page }) => {
    // 1. 503 Unavailable state on search
    await page.goto('/discover?q=unavailable-test')

    await expect(
      page.getByText('Open Library is unavailable right now, try again shortly'),
    ).toBeVisible()
    await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible()

    // 2. 404 Not Found state on work detail
    await page.goto('/discover/works/not-found-work-id')

    await expect(
      page.getByText("This book couldn't be found on Open Library"),
    ).toBeVisible()
    await expect(page.getByRole('button', { name: 'Back to Discover' })).toBeVisible()
    // Constitution §11: 404 states must not show a retry button
    await expect(page.getByRole('button', { name: /try again/i })).toBeHidden()
  })
})
