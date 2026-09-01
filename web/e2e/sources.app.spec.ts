import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

function axe(page: import('@playwright/test').Page) {
  return new AxeBuilder({ page }).disableRules(['color-contrast'])
}

test.describe('Phase 08: Sources E2E Walkthrough (FR-1 to FR-7)', () => {
  test('sources index screen, cards, and axe a11y audit', async ({ page }) => {
    await page.goto('/sources')

    // Header and action
    await expect(page.getByRole('heading', { name: 'Sources', level: 1 })).toBeVisible()
    const addBtn = page.getByRole('button', { name: '+ Add source' })
    await expect(addBtn).toBeVisible()

    // Source cards rendered from MSW fixture
    await expect(page.getByText('Personal OPDS').first()).toBeVisible()

    // Axe audit on /sources
    const indexAxe = await axe(page).analyze()
    expect(indexAxe.violations).toEqual([])
  })

  test('add source modal and validation with axe audit', async ({ page }) => {
    await page.goto('/sources')

    const addBtn = page.getByRole('button', { name: '+ Add source' })
    await addBtn.click()

    // Modal dialog
    const dialog = page.getByRole('dialog', { name: 'Add source' })
    await expect(dialog).toBeVisible()

    // Form inputs
    const labelInput = dialog.getByLabel('Source label')
    await expect(labelInput).toBeVisible()

    // Axe audit on open modal
    const modalAxe = await axe(page).analyze()
    expect(modalAxe.violations).toEqual([])

    // Switch to OPDS catalog
    await dialog.getByRole('radio', { name: 'OPDS catalog' }).click()
    const urlInput = dialog.getByLabel('Catalog base URL')
    await expect(urlInput).toBeVisible()

    // Fill form and submit
    await labelInput.fill('Project Gutenberg')
    await urlInput.fill('https://m.gutenberg.org/ebooks.opds/')
    await dialog.getByRole('button', { name: 'Add source' }).click()

    // Dialog closes
    await expect(dialog).toBeHidden()
  })

  test('source browse view with candidates and axe audit', async ({ page }) => {
    await page.goto('/sources')

    // Click Browse link on source card
    const browseLink = page.getByRole('link', { name: 'Browse →' }).first()
    await expect(browseLink).toBeVisible()
    await browseLink.click()

    // URL is /sources/:id
    await expect(page).toHaveURL(/\/sources\//)

    // Candidates list rendered
    const candidateGrid = page.getByTestId('source-candidate-grid')
    await expect(candidateGrid).toBeVisible()

    // Axe audit on browse view
    const browseAxe = await axe(page).analyze()
    expect(browseAxe.violations).toEqual([])

    // Navigate back to All sources
    const backLink = page.getByRole('link', { name: '← All sources' })
    await expect(backLink).toBeVisible()
    await backLink.click()
    await expect(page).toHaveURL(/\/sources$/)
  })

  test('remove source confirmation dialog flow', async ({ page }) => {
    await page.goto('/sources')

    const removeBtn = page.getByRole('button', { name: 'Remove' }).first()
    await expect(removeBtn).toBeVisible()
    await removeBtn.click()

    // Confirmation dialog
    const confirmDialog = page.getByRole('dialog', { name: 'Remove source' })
    await expect(confirmDialog).toBeVisible()
    await expect(confirmDialog).toContainText('Are you sure you want to remove')

    // Cancel closes dialog
    const cancelBtn = confirmDialog.getByRole('button', { name: 'Cancel' })
    await cancelBtn.click()
    await expect(confirmDialog).toBeHidden()
  })
})
