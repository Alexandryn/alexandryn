import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { renderWithProviders } from '../../test/renderWithProviders'
import { Import } from './Import'

const mockPendingCandidate = {
  id: 'cand-1',
  sourceId: 'src-1',
  fileReference: { id: 'dune.epub', format: 'EPUB', sizeBytes: 1048576 },
  status: 'pending',
  extractedMetadata: {
    title: 'Dune',
    authors: ['Frank Herbert'],
    isbn: '9780441172719',
    publisher: 'Chilton Books',
  },
  matchCandidates: [
    {
      type: 'open_library_work',
      confidence: 'high' as const,
      title: 'Dune',
      author: 'Frank Herbert',
      openLibraryWorkKey: 'OL893415W',
    },
  ],
  createdAt: '2026-09-01T12:00:00Z',
  updatedAt: '2026-09-01T12:00:00Z',
}

const mockFailedCandidate = {
  id: 'cand-failed-1',
  sourceId: 'src-1',
  fileReference: { id: 'corrupt.pdf', format: 'PDF', sizeBytes: 50000 },
  status: 'failed',
  lastError: 'pdf parser error: invalid header',
  createdAt: '2026-09-01T12:00:00Z',
  updatedAt: '2026-09-01T12:00:00Z',
}

describe('Import Screen (frontend-import-confirmation.md)', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders pending candidates with extracted info, match confidence badge, and actions (FR-2, FR-3)', async () => {
    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'pending') {
          return HttpResponse.json({ candidates: [mockPendingCandidate] })
        }
        if (status === 'queued') {
          return HttpResponse.json({ candidates: [] })
        }
        if (status === 'failed') {
          return HttpResponse.json({ candidates: [] })
        }
        return HttpResponse.json({ candidates: [] })
      }),
    )

    const { container } = renderWithProviders(<Import />, {
      routerEntries: ['/import'],
    })

    expect(
      await screen.findByRole('heading', { name: 'Import review', level: 1 }),
    ).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Dune', level: 3 })).toBeInTheDocument()
    expect(screen.getAllByText('Frank Herbert').length).toBeGreaterThan(0)
    expect(screen.getByText('High confidence')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Use this match' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Add from extracted info' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reject' })).toBeInTheDocument()

    expectNoAxeViolations(await runAxe(container))
  })

  it('shows in-flight processing banner when candidates are queued (FR-6)', async () => {
    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'queued') {
          return HttpResponse.json({
            candidates: [{ id: 'q-1' }, { id: 'q-2' }],
          })
        }
        return HttpResponse.json({ candidates: [] })
      }),
    )

    renderWithProviders(<Import />, {
      routerEntries: ['/import'],
    })

    expect(await screen.findByText(/Processing 2 files in the background/i)).toBeInTheDocument()
    expect(screen.getAllByRole('status').length).toBeGreaterThan(0)
  })

  it('confirms match and removes card on success (FR-3, FR-4)', async () => {
    let confirmCalled = false
    let capturedBody: unknown = null

    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'pending') {
          return HttpResponse.json({
            candidates: confirmCalled ? [] : [mockPendingCandidate],
          })
        }
        return HttpResponse.json({ candidates: [] })
      }),
      http.post('*/api/v1/import/candidates/cand-1/confirm', async ({ request }) => {
        confirmCalled = true
        capturedBody = await request.json()
        return HttpResponse.json({ ...mockPendingCandidate, status: 'confirmed' })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Import />, {
      routerEntries: ['/import'],
    })

    const useMatchBtn = await screen.findByRole('button', { name: 'Use this match' })
    await user.click(useMatchBtn)

    await waitFor(() => {
      expect(confirmCalled).toBe(true)
      expect(capturedBody).toEqual({
        action: 'use_open_library_match',
        openLibraryWorkKey: 'OL893415W',
      })
      expect(screen.getByText('No pending imports')).toBeInTheDocument()
    })
  })

  it('rejects candidate and removes card on success (FR-3, FR-4)', async () => {
    let rejectCalled = false

    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'pending') {
          return HttpResponse.json({
            candidates: rejectCalled ? [] : [mockPendingCandidate],
          })
        }
        return HttpResponse.json({ candidates: [] })
      }),
      http.post('*/api/v1/import/candidates/cand-1/reject', () => {
        rejectCalled = true
        return HttpResponse.json({ ...mockPendingCandidate, status: 'rejected' })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Import />, {
      routerEntries: ['/import'],
    })

    const rejectBtn = await screen.findByRole('button', { name: 'Reject' })
    await user.click(rejectBtn)

    // Confirm dialog (audit 0016 #241)
    const confirmBtn = await screen.findByRole('button', { name: 'Reject candidate' })
    await user.click(confirmBtn)

    await waitFor(() => {
      expect(rejectCalled).toBe(true)
      expect(screen.getByText('No pending imports')).toBeInTheDocument()
    })
  })

  it('canceling rejection modal keeps candidate intact (audit 0016 #241)', async () => {
    let rejectCalled = false

    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'pending') {
          return HttpResponse.json({
            candidates: [mockPendingCandidate],
          })
        }
        return HttpResponse.json({ candidates: [] })
      }),
      http.post('*/api/v1/import/candidates/cand-1/reject', () => {
        rejectCalled = true
        return HttpResponse.json({ ...mockPendingCandidate, status: 'rejected' })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Import />, {
      routerEntries: ['/import'],
    })

    const rejectBtn = await screen.findByRole('button', { name: 'Reject' })
    await user.click(rejectBtn)

    const cancelBtn = await screen.findByRole('button', { name: 'Cancel' })
    await user.click(cancelBtn)

    expect(rejectCalled).toBe(false)
    expect(screen.getAllByText('Dune').length).toBeGreaterThanOrEqual(1)
  })

  it('renders failed candidates section and dismisses to localStorage (FR-5)', async () => {
    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'failed') {
          return HttpResponse.json({ candidates: [mockFailedCandidate] })
        }
        return HttpResponse.json({ candidates: [] })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Import />, {
      routerEntries: ['/import'],
    })

    expect(await screen.findByText(/Couldn’t be imported/i)).toBeInTheDocument()
    expect(screen.getByText('corrupt.pdf')).toBeInTheDocument()
    expect(
      screen.getByText('This file appears to be damaged or is not a valid EPUB/PDF/CBZ file.'),
    ).toBeInTheDocument()

    const dismissBtn = screen.getByRole('button', { name: 'Dismiss' })
    await user.click(dismissBtn)

    await waitFor(() => {
      expect(screen.queryByText('corrupt.pdf')).not.toBeInTheDocument()
    })

    const stored = JSON.parse(localStorage.getItem('alexandryn_dismissed_import_failures') || '[]')
    expect(stored).toContain('cand-failed-1')
  })

  it('bounds dismissed import failure IDs in localStorage to at most 100 entries (audit 0016 #229)', async () => {
    const existing = Array.from({ length: 100 }, (_, i) => `old-failed-${i}`)
    localStorage.setItem('alexandryn_dismissed_import_failures', JSON.stringify(existing))

    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const url = new URL(request.url)
        const status = url.searchParams.get('status')
        if (status === 'failed') {
          return HttpResponse.json({ candidates: [mockFailedCandidate] })
        }
        return HttpResponse.json({ candidates: [] })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Import />, { routerEntries: ['/import'] })

    const dismissBtn = await screen.findByRole('button', { name: 'Dismiss' })
    await user.click(dismissBtn)

    await waitFor(() => {
      expect(screen.queryByText('corrupt.pdf')).not.toBeInTheDocument()
    })

    const stored = JSON.parse(localStorage.getItem('alexandryn_dismissed_import_failures') || '[]')
    expect(stored).toHaveLength(100)
    expect(stored).toContain('cand-failed-1')
    expect(stored).not.toContain('old-failed-0')
  })

  // audit 0016 #171: a candidate cover is extracted from an untrusted
  // book file; a disallowed data: URI (svg, html, …) must not reach an
  // <img src>, an allowed raster one may.
  it('only renders a candidate cover for an allowed image data URI', async () => {
    const withCover = (bytes: string) => ({
      ...mockPendingCandidate,
      id: `cand-${bytes.slice(5, 20)}`,
      extractedMetadata: { ...mockPendingCandidate.extractedMetadata, coverBytes: bytes },
    })

    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const status = new URL(request.url).searchParams.get('status')
        if (status === 'pending') {
          return HttpResponse.json({
            candidates: [
              withCover('data:image/svg+xml,<svg onload=alert(1)></svg>'),
              withCover('data:image/png;base64,iVBORw0KGgo='),
            ],
          })
        }
        return HttpResponse.json({ candidates: [] })
      }),
    )

    renderWithProviders(<Import />, { routerEntries: ['/import'] })
    await screen.findByRole('heading', { name: 'Import review', level: 1 })
    await waitFor(() =>
      expect(screen.getAllByRole('heading', { name: 'Dune', level: 3 })).toHaveLength(2),
    )

    const covers = screen.getAllByRole('img', { name: 'Cover for Dune' })
    expect(covers).toHaveLength(1)
    expect(covers[0]!.getAttribute('src')).toBe('data:image/png;base64,iVBORw0KGgo=')
  })

  it('serves candidate cover via dedicated URL with lazy loading when no inline cover is present', async () => {
    server.use(
      http.get('*/api/v1/import/candidates', ({ request }) => {
        const status = new URL(request.url).searchParams.get('status')
        if (status === 'pending') {
          return HttpResponse.json({
            candidates: [mockPendingCandidate],
          })
        }
        return HttpResponse.json({ candidates: [] })
      }),
    )

    renderWithProviders(<Import />, { routerEntries: ['/import'] })
    await screen.findByRole('heading', { name: 'Import review', level: 1 })

    const cover = await screen.findByRole('img', { name: 'Cover for Dune' })
    expect(cover.getAttribute('src')).toBe('/api/v1/import/candidates/cand-1/cover')
    expect(cover.getAttribute('loading')).toBe('lazy')
  })
})
