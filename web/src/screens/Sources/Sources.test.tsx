import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { Sources } from './Sources'
import type { SourceCreate } from '../../data/sources'

const mockSources = [
  {
    id: 's-1',
    label: 'Personal OPDS',
    kind: 'opds',
    config: { baseUrl: 'https://opds.example.org/catalog' },
    hasCredential: true,
    health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
    capabilities: { canList: true, canSearch: true, canDownload: true },
  },
  {
    id: 's-2',
    label: 'Local Calibre',
    kind: 'local-folder',
    config: { basePath: '/srv/books/calibre' },
    hasCredential: false,
    health: { status: 'unreachable', checkedAt: '2026-08-31T12:00:00Z', detail: 'path-not-found' },
    capabilities: { canList: true, canSearch: false, canDownload: true },
  },
]

describe('Sources Screen (FR-1, FR-3, FR-7)', () => {
  it('renders empty state when there are no sources', async () => {
    server.use(http.get('*/api/v1/sources', () => HttpResponse.json({ sources: [] })))

    renderWithProviders(<Sources />, { routerEntries: ['/sources'] })

    expect(await screen.findByText('No sources configured')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '+ Add source' })).toBeInTheDocument()
  })

  it('renders source cards with health badge, capabilities and browse link', async () => {
    server.use(http.get('*/api/v1/sources', () => HttpResponse.json({ sources: mockSources })))

    renderWithProviders(<Sources />, { routerEntries: ['/sources'] })

    expect(await screen.findByText('Personal OPDS')).toBeInTheDocument()
    expect(screen.getByText('Local Calibre')).toBeInTheDocument()
    expect(screen.getByText('Reachable')).toBeInTheDocument()
    expect(screen.getByText('Folder not found')).toBeInTheDocument()
    expect(screen.getByText('Search')).toBeInTheDocument()
    expect(screen.getAllByText('Browse →')).toHaveLength(2)
  })

  it('opens create modal, submits new source, and closes modal on success', async () => {
    let createdPayload: SourceCreate | null = null

    server.use(
      http.get('*/api/v1/sources', () => HttpResponse.json({ sources: mockSources })),
      http.post('*/api/v1/sources', async ({ request }) => {
        createdPayload = (await request.json()) as SourceCreate
        return HttpResponse.json(
          {
            id: 's-new',
            label: createdPayload.label,
            kind: createdPayload.kind,
            config: createdPayload.config,
            hasCredential: false,
            health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
            capabilities: { canList: true, canSearch: false, canDownload: true },
          },
          { status: 201 },
        )
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Sources />, { routerEntries: ['/sources'] })

    await screen.findByText('Personal OPDS')

    const addBtn = screen.getByRole('button', { name: '+ Add source' })
    await user.click(addBtn)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Add source', level: 2 })).toBeInTheDocument()

    await user.type(screen.getByLabelText('Source label'), 'New Folder Source')
    await user.type(screen.getByLabelText('Folder path'), '/home/user/books')

    const submitBtn = screen.getByRole('button', { name: 'Add source' })
    await user.click(submitBtn)

    await waitFor(() => {
      expect(createdPayload).toEqual({
        label: 'New Folder Source',
        kind: 'local-folder',
        config: { basePath: '/home/user/books' },
      })
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  it('opens remove confirmation dialog and deletes source on confirm (FR-7)', async () => {
    let deletedId: string | null = null

    server.use(
      http.get('*/api/v1/sources', () => HttpResponse.json({ sources: mockSources })),
      http.delete('*/api/v1/sources/:id', ({ params }) => {
        deletedId = params.id as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    const user = userEvent.setup()
    renderWithProviders(<Sources />, { routerEntries: ['/sources'] })

    await screen.findByText('Personal OPDS')

    const removeButtons = screen.getAllByRole('button', { name: 'Remove' })
    await user.click(removeButtons[0]!)

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Remove source', level: 2 })).toBeInTheDocument()
    expect(
      screen.getByText(/Are you sure you want to remove "Personal OPDS"?/i),
    ).toBeInTheDocument()

    const confirmBtn = screen.getByRole('button', { name: 'Remove source' })
    await user.click(confirmBtn)

    await waitFor(() => {
      expect(deletedId).toBe('s-1')
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })
  })

  // audit 0016 #142: a failed delete used to be swallowed — the dialog
  // stayed open with no feedback.
  it('keeps the dialog open and shows an error when delete fails', async () => {
    server.use(
      http.get('*/api/v1/sources', () => HttpResponse.json({ sources: mockSources })),
      http.delete('*/api/v1/sources/:id', () =>
        HttpResponse.json(
          { code: 'conflict', message: 'source is in use by a running import' },
          { status: 409 },
        ),
      ),
    )

    const user = userEvent.setup()
    renderWithProviders(<Sources />, { routerEntries: ['/sources'] })
    await screen.findByText('Personal OPDS')

    await user.click(screen.getAllByRole('button', { name: 'Remove' })[0]!)
    await user.click(screen.getByRole('button', { name: 'Remove source' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('running import')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument()
  })
})
