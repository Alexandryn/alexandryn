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

describe('Discover Screen (FR-1, FR-2, FR-3, FR-5)', () => {
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

  it('renders degraded 503 state with retry button when Open Library is unavailable (FR-5)', async () => {
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

  it('handles pagination next and previous buttons (FR-3)', async () => {
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
})
