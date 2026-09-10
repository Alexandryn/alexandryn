import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Phase 06: Library and Collections E2E Walkthrough (L21)', () => {
  test('library screen browsing, searching, filtering, and view mode toggling', async ({
    page,
  }) => {
    await page.goto('/library')

    // Library Header & Content
    await expect(page.getByRole('heading', { name: 'Library', level: 1 })).toBeVisible()
    // The in-page library filter — a textbox, distinct from the titlebar's
    // global searchbox which also carries a "Search library…" label.
    await expect(page.getByRole('textbox', { name: 'Search library' })).toBeVisible()
    await expect(page.getByRole('link', { name: /Middlemarch/ })).toBeVisible()

    // Test Search input debounce & URL update
    const searchInput = page.getByRole('textbox', { name: 'Search library' })
    await searchInput.fill('Middlemarch')
    await expect(page).toHaveURL(/q=Middlemarch/)

    // Clear search and verify query param removal
    await searchInput.fill('')
    await expect(page).not.toHaveURL(/q=/)

    // Test Filter segmented control
    const ownedFilterBtn = page.getByRole('radio', { name: 'Owned' })
    await ownedFilterBtn.click()
    await expect(page).toHaveURL(/filter=owned/)

    const allFilterBtn = page.getByRole('radio', { name: 'All' })
    await allFilterBtn.click()
    await expect(page).not.toHaveURL(/filter=/)

    // Test Grid / List view toggle (SegmentedControl renders radios)

    const listViewBtn = page.getByRole('radio', { name: 'List' })
    await listViewBtn.click()
    const storedView = await page.evaluate(() =>
      localStorage.getItem('alexandryn:library-view'),
    )
    expect(storedView).toBe('list')

    const gridViewBtn = page.getByRole('radio', { name: 'Grid' })
    await gridViewBtn.click()
    const storedGridView = await page.evaluate(() =>
      localStorage.getItem('alexandryn:library-view'),
    )
    expect(storedGridView).toBe('grid')

    // Axe audit on Library screen
    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])
  })

  test('work detail screen, metadata display, and manage collections flow', async ({
    page,
  }) => {
    await page.goto('/book/01JXXXXXXXXXXXXXXXXXXXXXXX')

    // Work Detail Heading
    await expect(
      page.getByRole('heading', { name: 'Middlemarch', level: 1 }),
    ).toBeVisible()
    await expect(page.getByText('George Eliot').first()).toBeVisible()
    await expect(page.getByText('Penguin Classics')).toBeVisible()

    // Manage Collections button opens modal
    const addColBtn = page.getByRole('button', { name: '+ Add to collection' })
    await expect(addColBtn).toBeVisible()
    await addColBtn.click()

    const dialog = page.getByRole('dialog', { name: 'Add to collection' })
    await expect(dialog).toBeVisible()

    // Axe audit inside modal
    const results = await axe(page).analyze()
    expect(results.violations).toEqual([])

    // Close modal
    await dialog.getByRole('button', { name: 'Done' }).click()
    await expect(dialog).toBeHidden()
  })

  test('collections index and collection detail screens', async ({ page }) => {
    await page.goto('/collections')

    await expect(
      page.getByRole('heading', { name: 'Collections', level: 1 }),
    ).toBeVisible()
    await expect(page.getByRole('link', { name: /Classics/ })).toBeVisible()

    // Axe audit on Collections Index
    const indexResults = await axe(page).analyze()
    expect(indexResults.violations).toEqual([])

    // Open Collection Detail
    await page.getByRole('link', { name: /Classics/ }).click()
    await expect(page).toHaveURL(/\/collections\/01JXXXXXXXXXXXXXXXXXXXXXXZ$/)
    await expect(
      page.getByRole('heading', { name: 'Classics', level: 1 }),
    ).toBeVisible()
    await expect(page.getByRole('link', { name: /Middlemarch/ })).toBeVisible()

    // Check Rename button opens rename modal
    await page.getByRole('button', { name: 'Rename' }).click()
    const renameDialog = page.getByRole('dialog', { name: 'Rename collection' })
    await expect(renameDialog).toBeVisible()
    await renameDialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(renameDialog).toBeHidden()

    // Check Delete button opens delete confirmation modal (Constitution §11)
    await page.getByRole('button', { name: 'Delete' }).click()
    const deleteDialog = page.getByRole('dialog', { name: 'Delete collection' })
    await expect(deleteDialog).toBeVisible()
    await expect(deleteDialog).toContainText('The books in it will stay in your library')
    await expect(deleteDialog.getByRole('button', { name: 'Delete collection' })).toBeVisible()
    await deleteDialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(deleteDialog).toBeHidden()


    // Axe audit on Collection Detail
    const detailResults = await axe(page).analyze()
    expect(detailResults.violations).toEqual([])
  })
})
