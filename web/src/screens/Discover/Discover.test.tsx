import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { Discover } from './Discover'

function renderWithProviders(initialEntries = ['/discover']) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route path="/discover" element={<Discover />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('Discover Screen', () => {
  it('renders initial idle state when no search query is present', () => {
    renderWithProviders()

    expect(screen.getByRole('heading', { name: 'Discover', level: 1 })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Search Open Library' })).toBeInTheDocument()
    expect(screen.getByText(/Search millions of books by title/)).toBeInTheDocument()
  })

  it('renders search results when query parameter is present in URL', async () => {
    server.use(
      http.get('*/api/v1/discover', () => {
        return HttpResponse.json({
          items: [
            {
              openLibraryWorkKey: 'OL82563W',
              title: 'Middlemarch',
              authors: [{ name: 'George Eliot' }],
              firstPublishYear: 1871,
              editionCount: 42,
            },
          ],
          total: 1,
          limit: 20,
          offset: 0,
        })
      }),
    )

    renderWithProviders(['/discover?q=Middlemarch'])

    await waitFor(() => {
      expect(screen.getAllByText('Middlemarch').length).toBeGreaterThanOrEqual(1)
    })
    expect(screen.getAllByText('George Eliot').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Showing 1 to 1 of 1 results')).toBeInTheDocument()
  })

  it('renders empty state when search returns zero results', async () => {
    server.use(
      http.get('*/api/v1/discover', () => {
        return HttpResponse.json({
          items: [],
          total: 0,
          limit: 20,
          offset: 0,
        })
      }),
    )

    renderWithProviders(['/discover?q=UnknownBookXYZ'])

    await waitFor(() => {
      expect(screen.getByText('No results found')).toBeInTheDocument()
    })
    expect(screen.getByText(/No books matched "UnknownBookXYZ"/)).toBeInTheDocument()
  })

  it('renders degraded 503 state with retry button when Open Library is unavailable', async () => {
    server.use(
      http.get('*/api/v1/discover', () => {
        return HttpResponse.json(
          {
            code: 'unavailable',
            message: 'Open Library is temporarily unavailable',
            correlationId: 'test-corr-123',
          },
          { status: 503 },
        )
      }),
    )

    renderWithProviders(['/discover?q=Middlemarch'])

    await waitFor(() => {
      expect(
        screen.getByText('Open Library is unavailable right now, try again shortly'),
      ).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })

  it('handles pagination next and previous buttons', async () => {
    server.use(
      http.get('*/api/v1/discover', ({ request }) => {
        const url = new URL(request.url)
        const offset = parseInt(url.searchParams.get('offset') ?? '0', 10)
        return HttpResponse.json({
          items: [
            {
              openLibraryWorkKey: `OL${offset}W`,
              title: `Book at offset ${offset}`,
              authors: [{ name: 'Author' }],
              editionCount: 1,
            },
          ],
          total: 50,
          limit: 20,
          offset,
        })
      }),
    )

    renderWithProviders(['/discover?q=fiction'])

    await waitFor(() => {
      expect(screen.getAllByText('Book at offset 0').length).toBeGreaterThanOrEqual(1)
    })

    const prevButton = screen.getByRole('button', { name: 'Previous page' })
    const nextButton = screen.getByRole('button', { name: 'Next page' })

    expect(prevButton).toBeDisabled()
    expect(nextButton).toBeEnabled()

    fireEvent.click(nextButton)

    await waitFor(() => {
      expect(screen.getAllByText('Book at offset 20').length).toBeGreaterThanOrEqual(1)
    })
  })

  // Paging replaces the whole result list, unmounting the
  // button that had focus; focus must land on the results, not document root.
  it('moves keyboard focus to the results region after a page change', async () => {
    server.use(
      http.get('*/api/v1/discover', ({ request }) => {
        const offset = parseInt(new URL(request.url).searchParams.get('offset') ?? '0', 10)
        return HttpResponse.json({
          items: [
            {
              openLibraryWorkKey: `OL${offset}W`,
              title: `Book at offset ${offset}`,
              authors: [{ name: 'Author' }],
              editionCount: 1,
            },
          ],
          total: 50,
          limit: 20,
          offset,
        })
      }),
    )

    renderWithProviders(['/discover?q=fiction'])
    await screen.findAllByText('Book at offset 0')

    // Not stolen on the first render.
    expect(screen.getByRole('region', { name: /showing 1 to 20 of 50/i })).not.toHaveFocus()

    fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findAllByText('Book at offset 20')

    await waitFor(() =>
      expect(screen.getByRole('region', { name: /showing 21 to 40 of 50/i })).toHaveFocus(),
    )
  })

  it('falls back to the page heading when the next page errors', async () => {
    let call = 0
    server.use(
      http.get('*/api/v1/discover', ({ request }) => {
        const offset = parseInt(new URL(request.url).searchParams.get('offset') ?? '0', 10)
        call += 1
        if (offset > 0) {
          return HttpResponse.json({ code: 'unavailable' }, { status: 503 })
        }
        return HttpResponse.json({
          items: [{ openLibraryWorkKey: 'OL0W', title: 'First book', authors: [{ name: 'A' }], editionCount: 1 }],
          total: 50,
          limit: 20,
          offset,
        })
      }),
    )

    renderWithProviders(['/discover?q=fiction'])
    await screen.findAllByText('First book')

    fireEvent.click(screen.getByRole('button', { name: 'Next page' }))

    await waitFor(() =>
      expect(screen.getByRole('heading', { name: 'Discover', level: 1 })).toHaveFocus(),
    )
    expect(call).toBeGreaterThan(1)
  })
})
