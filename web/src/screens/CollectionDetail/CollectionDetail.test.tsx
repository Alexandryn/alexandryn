import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { CollectionDetail } from './CollectionDetail'

const mockDetail = {
  id: 'c-1',
  name: 'Victorian Classics',
  works: [
    {
      id: 'w-1',
      title: 'Middlemarch',
      subtitle: 'A Study of Provincial Life',
      authors: ['George Eliot'],
      isOwned: true,
      collections: [{ id: 'c-1', name: 'Victorian Classics' }],
      addedAt: '2026-01-15T10:00:00Z',
    },
    {
      id: 'w-2',
      title: 'Bleak House',
      subtitle: '',
      authors: ['Charles Dickens'],
      isOwned: false,
      collections: [{ id: 'c-1', name: 'Victorian Classics' }],
      addedAt: '2026-01-10T12:00:00Z',
    },
  ],
}

describe('CollectionDetail Screen', () => {
  it('renders collection title and member works', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', () => HttpResponse.json(mockDetail)),
    )

    renderWithProviders(<CollectionDetail />, {
      routerEntries: ['/collections/c-1'],
    })

    expect(await screen.findByRole('heading', { name: 'Victorian Classics', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('2 books')).toBeInTheDocument()
    expect(screen.getAllByText('Middlemarch').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Bleak House').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Wanted').length).toBeGreaterThan(0) // w-2 isWanted
  })

  it('renders empty collection state when collection has 0 works', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', () =>
        HttpResponse.json({ id: 'c-1', name: 'Empty Collection', works: [] }),
      ),
    )

    renderWithProviders(<CollectionDetail />, {
      routerEntries: ['/collections/c-1'],
    })

    expect(await screen.findByRole('heading', { name: 'Empty Collection', level: 1 })).toBeInTheDocument()
    expect(screen.getByText('This collection is empty')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Browse library' })).toBeInTheDocument()
  })

  it('renames collection via modal and updates screen on success', async () => {
    let patchedName: string | null = null

    server.use(
      http.get('*/api/v1/collections/:id', () => HttpResponse.json(mockDetail)),
      http.patch('*/api/v1/collections/:id', async ({ request }) => {
        const body = (await request.json()) as { name: string }
        patchedName = body.name
        return HttpResponse.json({ ...mockDetail, name: body.name })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<CollectionDetail />, {
      routerEntries: ['/collections/c-1'],
    })

    await screen.findByRole('heading', { name: 'Victorian Classics', level: 1 })

    const renameBtn = screen.getByRole('button', { name: 'Rename' })
    await user.click(renameBtn)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Rename collection', level: 2 })).toBeInTheDocument()

    const input = screen.getByLabelText('New name')
    await user.clear(input)
    await user.type(input, '19th Century Classics')

    const saveBtn = screen.getByRole('button', { name: 'Save name' })
    await user.click(saveBtn)

    await waitFor(() => {
      expect(patchedName).toBe('19th Century Classics')
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  it('opens delete confirmation modal with specific copy and deletes on confirmation', async () => {
    let deletedId: string | null = null

    server.use(
      http.get('*/api/v1/collections/:id', () => HttpResponse.json(mockDetail)),
      http.delete('*/api/v1/collections/:id', ({ params }) => {
        deletedId = params.id as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<CollectionDetail />, {
      routerEntries: ['/collections/c-1'],
    })

    await screen.findByRole('heading', { name: 'Victorian Classics', level: 1 })

    const deleteBtn = screen.getByRole('button', { name: 'Delete' })
    await user.click(deleteBtn)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    // Constitution §11 specific delete confirmation copy
    expect(
      screen.getByText('Delete "Victorian Classics"? The books in it will stay in your library.'),
    ).toBeInTheDocument()

    const confirmBtn = screen.getByRole('button', { name: 'Delete collection' })
    await user.click(confirmBtn)

    await waitFor(() => {
      expect(deletedId).toBe('c-1')
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  it('renders dedicated 404 state without retry button when collection not found', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', () =>
        HttpResponse.json(
          { code: 'not_found', message: 'no collection with that id', correlationId: 'corr-404' },
          { status: 404 },
        ),
      ),
    )

    renderWithProviders(<CollectionDetail />, {
      routerEntries: ['/collections/nonexistent-id'],
    })

    expect(await screen.findByText("This collection doesn't exist.")).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to Collections' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /try again/i })).not.toBeInTheDocument()
  })

  it('renders ErrorState with correlation ID on 5xx error', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', () =>
        HttpResponse.json(
          { code: 'unavailable', message: 'DB down', correlationId: 'corr-500' },
          { status: 503 },
        ),
      ),
    )

    renderWithProviders(<CollectionDetail />, {
      routerEntries: ['/collections/c-1'],
    })

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-500')
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
  })
})
