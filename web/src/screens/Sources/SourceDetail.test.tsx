import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { SourceDetail } from './SourceDetail'

const mockSource = {
  id: 'src-123',
  label: 'Science Fiction OPDS',
  kind: 'opds',
  config: { baseUrl: 'https://scifi.example.org/catalog' },
  hasCredential: false,
  health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
  capabilities: { canList: true, canSearch: true, canDownload: true },
}

const mockCandidates = [
  {
    title: 'The Dispossessed',
    author: 'Ursula K. Le Guin',
    fileReference: { referenceId: 'dispossessed.epub', format: 'EPUB', sizeBytes: 450000 },
    coverUrl: null,
  },
]

describe('SourceDetail Screen (FR-6)', () => {
  it('renders source header, search input when canSearch is true, and candidates', async () => {
    server.use(
      http.get('*/api/v1/sources/src-123', () => HttpResponse.json(mockSource)),
      http.get('*/api/v1/sources/src-123/browse', () =>
        HttpResponse.json({ items: mockCandidates, nextCursor: null }),
      ),
    )

    renderWithProviders(<SourceDetail />, {
      routerEntries: ['/sources/src-123'],
    })

    expect(
      await screen.findByRole('heading', { name: 'Science Fiction OPDS', level: 1 }),
    ).toBeInTheDocument()
    expect(screen.getByText('Reachable')).toBeInTheDocument()
    expect(screen.getByLabelText('Search this source')).toBeInTheDocument()
    expect(await screen.findAllByText('The Dispossessed')).toHaveLength(2)
  })

  it('debounces search input and switches to search results (FR-6)', async () => {
    let capturedSearchQuery: string | null = null

    server.use(
      http.get('*/api/v1/sources/src-123', () => HttpResponse.json(mockSource)),
      http.get('*/api/v1/sources/src-123/browse', () =>
        HttpResponse.json({ items: mockCandidates, nextCursor: null }),
      ),
      http.get('*/api/v1/sources/src-123/search', ({ request }) => {
        const url = new URL(request.url)
        capturedSearchQuery = url.searchParams.get('q')
        return HttpResponse.json({
          items: [
            {
              title: 'Neuromancer',
              author: 'William Gibson',
              fileReference: { referenceId: 'neuro.epub', format: 'EPUB', sizeBytes: 300000 },
              coverUrl: null,
            },
          ],
          nextCursor: null,
        })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<SourceDetail />, {
      routerEntries: ['/sources/src-123'],
    })

    await screen.findByRole('heading', { name: 'Science Fiction OPDS', level: 1 })

    const searchInput = screen.getByLabelText('Search this source')
    await user.type(searchInput, 'Gibson')

    await waitFor(() => {
      expect(capturedSearchQuery).toBe('Gibson')
      expect(screen.getByRole('heading', { name: 'Neuromancer', level: 2 })).toBeInTheDocument()
    })
  })

  it('renders ErrorState when source is not found', async () => {
    server.use(
      http.get('*/api/v1/sources/src-not-found', () =>
        HttpResponse.json(
          { code: 'not_found', message: 'Source not found', correlationId: 'corr-404' },
          { status: 404 },
        ),
      ),
    )

    renderWithProviders(<SourceDetail />, {
      routerEntries: ['/sources/src-not-found'],
    })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-404')
  })

  it('triggers discover import mutation on click and navigates', async () => {
    let discoverCalled = false
    server.use(
      http.get('*/api/v1/sources/src-123', () => HttpResponse.json(mockSource)),
      http.get('*/api/v1/sources/src-123/browse', () =>
        HttpResponse.json({ items: mockCandidates, nextCursor: null }),
      ),
      http.post('*/api/v1/import/discover', () => {
        discoverCalled = true
        return HttpResponse.json({ discoveredCount: 5, skippedCount: 0, jobIds: ['job-1'] }, { status: 202 })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<SourceDetail />, {
      routerEntries: ['/sources/src-123'],
    })

    const importBtn = await screen.findByRole('button', { name: 'Import from this source' })
    await user.click(importBtn)

    await waitFor(() => {
      expect(discoverCalled).toBe(true)
    })
  })
})
