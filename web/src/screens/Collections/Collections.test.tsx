import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { Collections } from './Collections'

const mockCollections = [
  { id: 'c-1', name: 'Victorian Classics', workCount: 12 },
  { id: 'c-2', name: 'Science Fiction', workCount: 5 },
]

describe('Collections Index Screen', () => {
  it('renders empty state when there are no collections', async () => {
    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: [] }),
      ),
    )

    renderWithProviders(<Collections />, { routerEntries: ['/collections'] })

    expect(await screen.findByText('No collections yet')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'New collection' })).toBeInTheDocument()
  })

  it('renders collection tiles with names and work counts', async () => {
    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
    )

    renderWithProviders(<Collections />, { routerEntries: ['/collections'] })

    expect(await screen.findByText('Victorian Classics')).toBeInTheDocument()
    expect(screen.getByText('12 books')).toBeInTheDocument()
    expect(screen.getByText('Science Fiction')).toBeInTheDocument()
    expect(screen.getByText('5 books')).toBeInTheDocument()
  })

  it('opens create modal, submits new collection, and closes modal on success', async () => {
    let createdName: string | null = null

    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
      http.post('*/api/v1/collections', async ({ request }) => {
        const body = (await request.json()) as { name: string }
        createdName = body.name
        return HttpResponse.json(
          { id: 'c-new', name: body.name, works: [] },
          { status: 201 },
        )
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Collections />, { routerEntries: ['/collections'] })

    await screen.findByText('Victorian Classics')

    const newBtn = screen.getByRole('button', { name: '+ New collection' })
    await user.click(newBtn)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'New collection', level: 2 })).toBeInTheDocument()

    const input = screen.getByLabelText('Collection name')
    await user.type(input, 'Philosophy')

    const submitBtn = screen.getByRole('button', { name: 'Create collection' })
    await user.click(submitBtn)

    await waitFor(() => {
      expect(createdName).toBe('Philosophy')
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  it('shows inline validation error and retains input if creation fails', async () => {
    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
      http.post('*/api/v1/collections', () =>
        HttpResponse.json(
          { code: 'invalid_input', message: 'name: contains control characters', correlationId: 'corr-1' },
          { status: 400 },
        ),
      ),
    )

    const user = userEvent.setup()
    renderWithProviders(<Collections />, { routerEntries: ['/collections'] })

    await screen.findByText('Victorian Classics')

    const newBtn = screen.getByRole('button', { name: '+ New collection' })
    await user.click(newBtn)

    const input = screen.getByLabelText('Collection name')
    await user.type(input, 'Duplicate Name')

    const submitBtn = screen.getByRole('button', { name: 'Create collection' })
    await user.click(submitBtn)

    expect(await screen.findByRole('alert')).toHaveTextContent('name: contains control characters')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(input).toHaveValue('Duplicate Name')
  })


  it('shows ErrorState on server failure', async () => {
    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json(
          { code: 'unavailable', message: 'Database connection failed', correlationId: 'corr-err' },
          { status: 503 },
        ),
      ),
    )

    renderWithProviders(<Collections />, { routerEntries: ['/collections'] })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-err')
  })
})
