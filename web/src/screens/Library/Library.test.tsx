import { screen } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { Library } from './Library'

describe('Library (FR-8)', () => {
  it('renders <EmptyState>, not a blank pane, for an empty successful response', async () => {
    server.use(http.get('*/api/v1/library', () => HttpResponse.json([])))
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect(await screen.findByText('Your library is waiting.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('renders the list, not <EmptyState>, when the response has items', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json([{ id: 'x', title: 'A Book', author: 'An Author' }]),
      ),
    )
    renderWithProviders(<Library />, { routerEntries: ['/library'] })

    expect(await screen.findByText('A Book')).toBeInTheDocument()
    expect(screen.getByRole('list')).toBeInTheDocument()
    expect(screen.queryByText('Your library is waiting.')).not.toBeInTheDocument()
  })

  it('shows the error state with its correlation ID when the fetch fails', async () => {
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
    expect(screen.queryByText('Your library is waiting.')).not.toBeInTheDocument()
  })
})
