import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { WorkDetail } from './WorkDetail'

const mockDetail = {
  id: '01JXXXXXXXXXXXXXXXXXXXXXXX',
  title: 'Middlemarch',
  subtitle: 'A Study of Provincial Life',
  authors: ['George Eliot'],
  subjects: ['Victorian Literature', 'Classic Fiction'],
  originalLanguage: 'en',
  ownedEditions: [
    {
      id: 'ed-1',
      language: 'en',
      isbn: '9780141439549',
      publisher: 'Penguin Classics',
      publicationYear: 2003,
      addedAt: '2026-01-15T10:00:00Z',
      formats: ['epub', 'pdf'],
    },
    {
      id: 'ed-2',
      language: 'en',
      isbn: '9780192834027',
      publisher: 'Oxford World Classics',
      publicationYear: 1998,
      addedAt: '2026-01-10T12:00:00Z',
      formats: ['pdf'],
    },
  ],
  collections: [
    {
      id: 'coll-1',
      name: 'Victorian Classics',
      addedAt: '2026-01-20T08:00:00Z',
    },
  ],
}

describe('WorkDetail Screen', () => {
  it('renders work metadata, owned editions, and collections', async () => {
    server.use(
      http.get('*/api/v1/works/:id', () => HttpResponse.json(mockDetail)),
    )

    renderWithProviders(<WorkDetail />, {
      routerEntries: ['/book/01JXXXXXXXXXXXXXXXXXXXXXXX'],
    })

    expect(await screen.findByRole('heading', { name: 'Middlemarch', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('A Study of Provincial Life')).toBeInTheDocument()
    expect(screen.getByText('George Eliot', { selector: 'strong' })).toBeInTheDocument()
    expect(screen.getByText('Victorian Literature')).toBeInTheDocument()
    expect(screen.getByText('Victorian Classics')).toBeInTheDocument()
    expect(screen.getByText('Penguin Classics')).toBeInTheDocument()
    expect(screen.getByText('Oxford World Classics')).toBeInTheDocument()
  })

  it('renders Read button only for editions with epub format', async () => {
    server.use(
      http.get('*/api/v1/works/:id', () => HttpResponse.json(mockDetail)),
    )

    renderWithProviders(<WorkDetail />, {
      routerEntries: ['/book/01JXXXXXXXXXXXXXXXXXXXXXXX'],
    })

    await screen.findByRole('heading', { name: 'Middlemarch', level: 1 })

    const readButtons = screen.getAllByTestId('read-edition-btn')
    expect(readButtons).toHaveLength(1)
    expect(readButtons[0]).toHaveAttribute(
      'href',
      '/read/01JXXXXXXXXXXXXXXXXXXXXXXX/ed-1',
    )
  })

  it('renders "Not yet in your library" when ownedEditions is empty', async () => {
    const wantedWork = {
      ...mockDetail,
      ownedEditions: [],
    }
    server.use(
      http.get('*/api/v1/works/:id', () => HttpResponse.json(wantedWork)),
    )

    renderWithProviders(<WorkDetail />, {
      routerEntries: ['/book/01JXXXXXXXXXXXXXXXXXXXXXXX'],
    })

    expect(await screen.findByRole('heading', { name: 'Middlemarch', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('Not yet in your library')).toBeInTheDocument()
  })

  it('renders dedicated 404 state without retry button when work is not found', async () => {
    server.use(
      http.get('*/api/v1/works/:id', () =>
        HttpResponse.json(
          { code: 'not_found', message: 'no work with that id', correlationId: 'corr-404' },
          { status: 404 },
        ),
      ),
    )

    renderWithProviders(<WorkDetail />, {
      routerEntries: ['/book/nonexistent-id'],
    })

    expect(await screen.findByText("This book isn't in your library.")).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to Library' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /try again/i })).not.toBeInTheDocument()
  })


  it('renders ErrorState with correlation ID and retry button on server error', async () => {
    server.use(
      http.get('*/api/v1/works/:id', () =>
        HttpResponse.json(
          { code: 'unavailable', message: 'Database unreachable', correlationId: 'corr-500' },
          { status: 503 },
        ),
      ),
    )

    renderWithProviders(<WorkDetail />, {
      routerEntries: ['/book/01JXXXXXXXXXXXXXXXXXXXXXXX'],
    })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-500')
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })

  it('supports WAI-ARIA accessible tab switching and keyboard arrow navigation', async () => {
    server.use(
      http.get('*/api/v1/works/:id', () => HttpResponse.json(mockDetail)),
    )

    const user = userEvent.setup()
    renderWithProviders(<WorkDetail />, {
      routerEntries: ['/book/01JXXXXXXXXXXXXXXXXXXXXXXX'],
    })

    await screen.findByRole('heading', { name: 'Middlemarch', level: 1 })

    const tablist = screen.getByRole('tablist', { name: 'Book detail sections' })
    expect(tablist).toBeInTheDocument()

    const aboutTab = screen.getByRole('tab', { name: /About/i })
    const editionsTab = screen.getByRole('tab', { name: /Editions/i })
    const sourcesTab = screen.getByRole('tab', { name: /Sources/i })

    expect(aboutTab).toHaveAttribute('aria-selected', 'true')
    expect(editionsTab).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByRole('tabpanel', { name: 'About' })).toBeInTheDocument()

    // Click Editions tab
    await user.click(editionsTab)
    expect(editionsTab).toHaveAttribute('aria-selected', 'true')
    expect(aboutTab).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByRole('tabpanel', { name: /Editions/i })).toBeInTheDocument()

    // Arrow navigation: press ArrowRight on Editions tab -> moves to Sources tab
    editionsTab.focus()
    await user.keyboard('{ArrowRight}')
    expect(sourcesTab).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByRole('tabpanel', { name: /Sources/i })).toBeInTheDocument()

    // Arrow navigation: press ArrowLeft on Sources tab -> moves back to Editions
    await user.keyboard('{ArrowLeft}')
    expect(editionsTab).toHaveAttribute('aria-selected', 'true')
  })
})
