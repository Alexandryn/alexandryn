import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { server } from '../../mocks/node'
import { AddToCollectionModal } from './AddToCollectionModal'
import type { WorkDetail } from '../../data/library'

const mockWork: WorkDetail = {
  id: 'work-100',
  title: 'Middlemarch',
  subtitle: 'A Study of Provincial Life',
  authors: ['George Eliot'],
  subjects: ['Victorian'],
  originalLanguage: 'en',
  ownedEditions: [],
  collections: [{ id: 'c-1', name: 'Classics', addedAt: '2026-01-01T00:00:00Z' }],
}

const mockCollections = [
  { id: 'c-1', name: 'Classics', workCount: 1 },
  { id: 'c-2', name: 'Sci-Fi', workCount: 0 },
]

function renderModal(props?: { onOpenChange?: (open: boolean) => void }) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  const onOpenChange = props?.onOpenChange ?? vi.fn()

  const result = render(
    <QueryClientProvider client={queryClient}>
      <AddToCollectionModal
        open={true}
        onOpenChange={onOpenChange}
        work={mockWork}
      />
    </QueryClientProvider>,
  )

  return { ...result, onOpenChange }
}

describe('AddToCollectionModal (FR-4 & Reliability)', () => {
  it('renders existing collections and checks active memberships', async () => {
    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
    )

    renderModal()

    expect(await screen.findByText('Classics')).toBeInTheDocument()
    expect(screen.getByText('Sci-Fi')).toBeInTheDocument()

    const checkboxes = screen.getAllByRole('checkbox')
    expect(checkboxes[0]).toBeChecked() // c-1 (Classics) is member
    expect(checkboxes[1]).not.toBeChecked() // c-2 (Sci-Fi) is not member
  })

  it('toggles collection membership via add and remove endpoints', async () => {
    let addedWorkId: string | null = null
    let removedWorkId: string | null = null

    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
      http.post('*/api/v1/collections/:id/works', async ({ request }) => {
        const body = (await request.json()) as { workId: string }
        addedWorkId = body.workId
        return HttpResponse.json({ id: 'c-2', name: 'Sci-Fi', works: [] })
      }),
      http.delete('*/api/v1/collections/:id/works/:workId', ({ params }) => {
        removedWorkId = params.workId as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    const user = userEvent.setup()
    renderModal()

    expect(await screen.findByText('Classics')).toBeInTheDocument()
    const checkboxes = screen.getAllByRole('checkbox')

    // Toggle uncheck on Classics
    await user.click(checkboxes[0]!)
    await waitFor(() => expect(removedWorkId).toBe('work-100'))

    // Toggle check on Sci-Fi
    await user.click(checkboxes[1]!)
    await waitFor(() => expect(addedWorkId).toBe('work-100'))
  })


  it('sequentially creates collection then adds work on success', async () => {
    let createdName: string | null = null
    let addedToCollectionId: string | null = null

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
      http.post('*/api/v1/collections/:id/works', ({ params }) => {
        addedToCollectionId = params.id as string
        return HttpResponse.json({ id: params.id, name: 'Essays', works: [] })
      }),
    )

    const user = userEvent.setup()
    const { onOpenChange } = renderModal()

    await screen.findByText('Classics')

    const input = screen.getByLabelText('New collection name')
    await user.type(input, 'Essays')

    const createBtn = screen.getByRole('button', { name: 'Create & Add' })
    await user.click(createBtn)

    await waitFor(() => {
      expect(createdName).toBe('Essays')
      expect(addedToCollectionId).toBe('c-new')
      expect(onOpenChange).toHaveBeenCalledWith(false)
    })
  })

  it('case (a): handles create failure outright without attempting add', async () => {
    let addAttempted = false

    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
      http.post('*/api/v1/collections', () =>
        HttpResponse.json(
          { code: 'invalid_input', message: 'name: contains control characters', correlationId: 'c1' },
          { status: 400 },
        ),
      ),
      http.post('*/api/v1/collections/:id/works', () => {
        addAttempted = true
        return HttpResponse.json({})
      }),
    )

    const user = userEvent.setup()
    const { onOpenChange } = renderModal()

    await screen.findByText('Classics')

    const input = screen.getByLabelText('New collection name')
    await user.type(input, 'Bad Name')

    const createBtn = screen.getByRole('button', { name: 'Create & Add' })
    await user.click(createBtn)

    expect(await screen.findByText('name: contains control characters')).toBeInTheDocument()
    expect(addAttempted).toBe(false)
    expect(input).toHaveValue('Bad Name')
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it('case (b): handles partial failure when create succeeds but add fails', async () => {
    let retryAttempted = false

    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
      http.post('*/api/v1/collections', () =>
        HttpResponse.json(
          { id: 'c-created', name: 'Philosophy', works: [] },
          { status: 201 },
        ),
      ),
      http.post('*/api/v1/collections/:id/works', () => {
        if (!retryAttempted) {
          return HttpResponse.json(
            { code: 'not_found', message: 'no work with that id', correlationId: 'c2' },
            { status: 404 },
          )
        }
        return HttpResponse.json({ id: 'c-created', name: 'Philosophy', works: [] })
      }),
    )

    const user = userEvent.setup()
    const { onOpenChange } = renderModal()

    await screen.findByText('Classics')

    const input = screen.getByLabelText('New collection name')
    await user.type(input, 'Philosophy')

    const createBtn = screen.getByRole('button', { name: 'Create & Add' })
    await user.click(createBtn)

    // Partial failure banner appears with retry button
    expect(await screen.findByText(/was created, but adding this book failed/)).toBeInTheDocument()
    const retryBtn = screen.getByRole('button', { name: 'Try adding again' })
    retryAttempted = true
    await user.click(retryBtn)

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false)
    })
  })


  it('case (c): handles ambiguous outcome when create request is aborted', async () => {
    let addAttempted = false

    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: mockCollections }),
      ),
      http.post('*/api/v1/collections', () => {
        const err = new Error('The operation was aborted.')
        err.name = 'AbortError'
        return HttpResponse.error()
      }),
      http.post('*/api/v1/collections/:id/works', () => {
        addAttempted = true
        return HttpResponse.json({})
      }),
    )

    // Simulate rejection with AbortError
    const user = userEvent.setup()
    const { onOpenChange } = renderModal()

    await screen.findByText('Classics')

    const input = screen.getByLabelText('New collection name')
    await user.type(input, 'Timeout Collection')

    // Mock fetch rejection with AbortError for this test
    const originalFetch = window.fetch
    const abortErr = new Error('The user aborted a request.')
    abortErr.name = 'AbortError'
    window.fetch = vi.fn().mockImplementation((url) => {
      if (String(url).includes('/api/v1/collections') && !String(url).endsWith('/api/v1/collections')) {
        return originalFetch(url)
      }
      return Promise.reject(abortErr)
    })

    try {
      const createBtn = screen.getByRole('button', { name: 'Create & Add' })
      await user.click(createBtn)

      // Warning banner appears without automatic retry or add attempt
      expect(
        await screen.findByText('Unable to confirm — check your collections list.'),
      ).toBeInTheDocument()
      expect(addAttempted).toBe(false)

      // Close button closes modal
      const closeBtns = screen.getAllByRole('button', { name: 'Close' })
      expect(closeBtns.length).toBeGreaterThan(0)
      await user.click(closeBtns[0]!)
      expect(onOpenChange).toHaveBeenCalledWith(false)
    } finally {

      window.fetch = originalFetch
    }
  })
})

