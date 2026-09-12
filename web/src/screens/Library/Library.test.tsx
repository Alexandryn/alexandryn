import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { Library } from './Library'

const mockWorks = [
  {
    id: 'work-1',
    title: 'Middlemarch',
    subtitle: 'A Study of Provincial Life',
    authors: ['George Eliot'],
    isOwned: true,
    collections: [{ id: 'coll-1', name: 'Victorian Classics' }],
    addedAt: '2026-01-15T10:00:00Z',
  },
  {
    id: 'work-2',
    title: 'Dune',
    subtitle: '',
    authors: ['Frank Herbert'],
    isOwned: true,
    collections: [],
    addedAt: '2026-01-10T12:00:00Z',
  },
]

describe('Library Screen', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders <EmptyState>, not a blank pane, for an empty library', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json({ works: [], nextCursor: null }),
      ),
    )
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect(await screen.findByText('Your library is waiting.')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('renders filtered empty state with clear filters button when search/filter active', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json({ works: [], nextCursor: null }),
      ),
    )
    renderWithProviders(<Library />, { routerEntries: ['/library?q=nonexistent'] })

    expect(await screen.findByText('No books match your search')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Clear filters' })).toBeInTheDocument()
  })

  it('renders works in grid view by default', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json({ works: mockWorks, nextCursor: null }),
      ),
    )
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect((await screen.findAllByText('Middlemarch'))[0]).toBeInTheDocument()
    expect(screen.getAllByText('Dune')[0]).toBeInTheDocument()
  })


  it('switches view mode and persists to localStorage', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json({ works: mockWorks, nextCursor: null }),
      ),
    )
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect((await screen.findAllByText('Middlemarch'))[0]).toBeInTheDocument()

    const listToggle = screen.getByRole('radio', { name: 'List' })
    await user.click(listToggle)

    expect(localStorage.getItem('alexandryn:library-view')).toBe('list')
  })

  it('updates filter in URL and sends request with filter param', async () => {
    let capturedFilter: string | null = null
    server.use(
      http.get('*/api/v1/library', ({ request }) => {
        const url = new URL(request.url)
        capturedFilter = url.searchParams.get('filter')
        return HttpResponse.json({ works: mockWorks, nextCursor: null })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect((await screen.findAllByText('Middlemarch'))[0]).toBeInTheDocument()

    const ownedRadio = screen.getByRole('radio', { name: 'Owned' })
    await user.click(ownedRadio)

    await waitFor(() => {
      expect(capturedFilter).toBe('owned')
    })
  })

  it('debounces search input by 300ms before querying', async () => {
    let capturedQ: string | null = null
    server.use(
      http.get('*/api/v1/library', ({ request }) => {
        const url = new URL(request.url)
        capturedQ = url.searchParams.get('q')
        return HttpResponse.json({ works: mockWorks, nextCursor: null })
      }),
    )

    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect((await screen.findAllByText('Middlemarch'))[0]).toBeInTheDocument()

    const searchInput = screen.getByLabelText('Search library')
    fireEvent.change(searchInput, { target: { value: 'Middle' } })

    // Immediately after typing, capturedQ is not yet updated
    expect(capturedQ).toBeNull()

    // After debounce delay, capturedQ is updated
    await waitFor(
      () => {
        expect(capturedQ).toBe('Middle')
      },
      { timeout: 1000 },
    )
  })

  it('shows error state with its correlation ID when fetch fails', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json(
          { code: 'unavailable', message: 'Down for maintenance.', correlationId: 'corr-lib-1' },
          { status: 503 },
        ),
      ),
    )
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-lib-1')
  })
})
