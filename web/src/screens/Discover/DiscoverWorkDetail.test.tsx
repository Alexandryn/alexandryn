import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { DiscoverWorkDetail } from './DiscoverWorkDetail'

function renderWithProviders(initialEntries = ['/discover/works/OL82563W']) {
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
          <Route path="/discover/works/:openLibraryId" element={<DiscoverWorkDetail />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('DiscoverWorkDetail Screen', () => {
  it('renders work details, description, subjects, and editions', async () => {
    server.use(
      http.get('*/api/v1/discover/works/OL82563W', () => {
        return HttpResponse.json({
          work: {
            title: 'Middlemarch',
            subtitle: 'A Study of Provincial Life',
            description: 'A masterpiece of realist fiction.',
            subjects: ['Provincial life', 'Classics'],
            authors: [{ name: 'George Eliot' }],
            coverUrl: '/api/v1/discover/covers/8256301',
          },
          editions: [
            {
              title: 'Middlemarch (Penguin Classics)',
              publisher: 'Penguin',
              publishDate: '2003',
              language: 'en',
              openLibraryEditionKey: 'OL7353617M',
            },
          ],
        })
      }),
    )

    renderWithProviders(['/discover/works/OL82563W'])

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Middlemarch', level: 1 })).toBeInTheDocument()
    })

    expect(screen.getByText('A Study of Provincial Life')).toBeInTheDocument()
    expect(screen.getByText('By George Eliot')).toBeInTheDocument()
    expect(screen.getByText('A masterpiece of realist fiction.')).toBeInTheDocument()
    expect(screen.getByText('Provincial life')).toBeInTheDocument()
    expect(screen.getByText('Classics')).toBeInTheDocument()

    expect(screen.getByText('Editions (1)')).toBeInTheDocument()
    expect(screen.getByText('Middlemarch (Penguin Classics)')).toBeInTheDocument()
    expect(screen.getByText('Penguin')).toBeInTheDocument()
    expect(screen.getByText('OL7353617M')).toBeInTheDocument()
  })

  it('omits optional sections when description, subjects, or editions are missing', async () => {
    server.use(
      http.get('*/api/v1/discover/works/OLMinimalW', () => {
        return HttpResponse.json({
          work: {
            title: 'Minimal Book',
            authors: [{ name: 'Minimal Author' }],
          },
          editions: [],
        })
      }),
    )

    renderWithProviders(['/discover/works/OLMinimalW'])

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Minimal Book', level: 1 })).toBeInTheDocument()
    })

    expect(screen.queryByText('About this work')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Subjects')).not.toBeInTheDocument()
    expect(screen.queryByText(/Editions/)).not.toBeInTheDocument()
  })

  it('renders distinct 404 not-found state when work does not exist', async () => {
    server.use(
      http.get('*/api/v1/discover/works/OLNotFoundW', () => {
        return HttpResponse.json(
          {
            code: 'not_found',
            message: 'work not found on Open Library',
            correlationId: 'test-corr-404',
          },
          { status: 404 },
        )
      }),
    )

    renderWithProviders(['/discover/works/OLNotFoundW'])

    await waitFor(() => {
      expect(
        screen.getByText("This book couldn't be found on Open Library"),
      ).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Back to Discover' })).toBeInTheDocument()
  })

  it('renders distinct 503 degraded state when Open Library is unavailable', async () => {
    server.use(
      http.get('*/api/v1/discover/works/OLUnavailableW', () => {
        return HttpResponse.json(
          {
            code: 'unavailable',
            message: 'Open Library is temporarily unavailable',
            correlationId: 'test-corr-503',
          },
          { status: 503 },
        )
      }),
    )

    renderWithProviders(['/discover/works/OLUnavailableW'])

    await waitFor(() => {
      expect(
        screen.getByText('Open Library is unavailable right now, try again shortly'),
      ).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })
})
