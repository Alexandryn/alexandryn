import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import { server } from '../mocks/node'
import {
  checkSourceHealth,
  createSource,
  deleteSource,
  fetchSource,
  fetchSourceBrowse,
  fetchSources,
  fetchSourceSearch,
  updateSource,
  useCreateSource,
  useDeleteSource,
  useHealthCheckSource,
  useSource,
  useSourceCandidates,
  useSources,
  useUpdateSource,
} from './sources'

function createTestWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return {
    queryClient,
    wrapper: function TestWrapper({ children }: { children: ReactNode }) {
      return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    },
  }
}

describe('sources data layer (FR-1 to FR-7)', () => {
  it('fetchSources fetches list of sources', async () => {
    server.use(
      http.get('*/api/v1/sources', () =>
        HttpResponse.json({
          sources: [
            {
              id: 's-1',
              label: 'Local Books',
              kind: 'local-folder',
              config: { basePath: '/srv/books' },
              hasCredential: false,
              health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
              capabilities: { canList: true, canSearch: false, canDownload: true },
            },
          ],
        }),
      ),
    )

    const sources = await fetchSources()
    expect(sources).toHaveLength(1)
    expect(sources[0]?.label).toBe('Local Books')
    expect(sources[0]?.kind).toBe('local-folder')
  })

  it('fetchSource fetches single source detail', async () => {
    server.use(
      http.get('*/api/v1/sources/:id', ({ params }) => {
        expect(params.id).toBe('s-1')
        return HttpResponse.json({
          id: 's-1',
          label: 'OPDS Source',
          kind: 'opds',
          config: { baseUrl: 'https://opds.example.com' },
          hasCredential: true,
          health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
          capabilities: { canList: true, canSearch: true, canDownload: true },
        })
      }),
    )

    const src = await fetchSource('s-1')
    expect(src.id).toBe('s-1')
    expect(src.hasCredential).toBe(true)
    expect(src.capabilities.canSearch).toBe(true)
  })

  it('createSource sends POST with source create input', async () => {
    let capturedBody: unknown = null
    server.use(
      http.post('*/api/v1/sources', async ({ request }) => {
        capturedBody = await request.json()
        return HttpResponse.json(
          {
            id: 's-new',
            label: 'New Source',
            kind: 'local-folder',
            config: { basePath: '/srv/new' },
            hasCredential: false,
            health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
            capabilities: { canList: true, canSearch: false, canDownload: true },
          },
          { status: 201 },
        )
      }),
    )

    const created = await createSource({
      label: 'New Source',
      kind: 'local-folder',
      config: { basePath: '/srv/new' },
    })
    expect(capturedBody).toEqual({
      label: 'New Source',
      kind: 'local-folder',
      config: { basePath: '/srv/new' },
    })
    expect(created.id).toBe('s-new')
  })

  it('updateSource sends PATCH with partial updates', async () => {
    let capturedBody: unknown = null
    server.use(
      http.patch('*/api/v1/sources/:id', async ({ request, params }) => {
        expect(params.id).toBe('s-1')
        capturedBody = await request.json()
        return HttpResponse.json({
          id: 's-1',
          label: 'Updated Label',
          kind: 'opds',
          config: { baseUrl: 'https://opds.example.com' },
          hasCredential: true,
          health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
          capabilities: { canList: true, canSearch: true, canDownload: true },
        })
      }),
    )

    const updated = await updateSource('s-1', { label: 'Updated Label' })
    expect(capturedBody).toEqual({ label: 'Updated Label' })
    expect(updated.label).toBe('Updated Label')
  })

  it('deleteSource sends DELETE request', async () => {
    let deletedId: string | undefined
    server.use(
      http.delete('*/api/v1/sources/:id', ({ params }) => {
        deletedId = params.id as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    await deleteSource('s-1')
    expect(deletedId).toBe('s-1')
  })

  it('checkSourceHealth sends POST to health-check endpoint', async () => {
    server.use(
      http.post('*/api/v1/sources/:id/health-check', ({ params }) => {
        expect(params.id).toBe('s-1')
        return HttpResponse.json({
          status: 'reachable',
          checkedAt: '2026-08-31T12:00:00Z',
          detail: null,
        })
      }),
    )

    const health = await checkSourceHealth('s-1')
    expect(health.status).toBe('reachable')
  })

  it('fetchSourceBrowse and fetchSourceSearch query items with params', async () => {
    server.use(
      http.get('*/api/v1/sources/:id/browse', ({ request, params }) => {
        expect(params.id).toBe('s-1')
        const url = new URL(request.url)
        expect(url.searchParams.get('limit')).toBe('20')
        return HttpResponse.json({
          items: [
            {
              title: 'Book A',
              author: 'Author A',
              fileReference: { referenceId: 'ref-a', format: 'EPUB', sizeBytes: 1000 },
              coverUrl: null,
            },
          ],
          nextCursor: 'cursor-2',
        })
      }),
      http.get('*/api/v1/sources/:id/search', ({ request, params }) => {
        expect(params.id).toBe('s-1')
        const url = new URL(request.url)
        expect(url.searchParams.get('q')).toBe('earthsea')
        return HttpResponse.json({
          items: [
            {
              title: 'Earthsea',
              author: 'Ursula K. Le Guin',
              fileReference: { referenceId: 'ref-e', format: 'EPUB', sizeBytes: 2000 },
              coverUrl: null,
            },
          ],
          nextCursor: null,
        })
      }),
    )

    const browse = await fetchSourceBrowse('s-1', { limit: 20 })
    expect(browse.items).toHaveLength(1)
    expect(browse.nextCursor).toBe('cursor-2')

    const search = await fetchSourceSearch('s-1', { q: 'earthsea' })
    expect(search.items).toHaveLength(1)
    expect(search.items[0]?.title).toBe('Earthsea')
  })

  it('useSources, useSource, and mutation hooks handle caching and invalidation', async () => {
    const { queryClient, wrapper } = createTestWrapper()
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    server.use(
      http.get('*/api/v1/sources', () =>
        HttpResponse.json({
          sources: [
            {
              id: 's-1',
              label: 'Source 1',
              kind: 'local-folder',
              config: { basePath: '/srv/books' },
              hasCredential: false,
              health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
              capabilities: { canList: true, canSearch: false, canDownload: true },
            },
          ],
        }),
      ),
      http.get('*/api/v1/sources/s-1', () =>
        HttpResponse.json({
          id: 's-1',
          label: 'Source 1',
          kind: 'local-folder',
          config: { basePath: '/srv/books' },
          hasCredential: false,
          health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
          capabilities: { canList: true, canSearch: false, canDownload: true },
        }),
      ),
      http.post('*/api/v1/sources', () =>
        HttpResponse.json(
          {
            id: 's-2',
            label: 'Source 2',
            kind: 'opds',
            config: { baseUrl: 'https://opds.example.com' },
            hasCredential: false,
            health: { status: 'reachable', checkedAt: '2026-08-31T12:00:00Z', detail: null },
            capabilities: { canList: true, canSearch: true, canDownload: true },
          },
          { status: 201 },
        ),
      ),
    )

    const { result: listResult } = renderHook(() => useSources(), { wrapper })
    await waitFor(() => expect(listResult.current.isSuccess).toBe(true))

    const { result: detailResult } = renderHook(() => useSource('s-1'), { wrapper })
    await waitFor(() => expect(detailResult.current.isSuccess).toBe(true))

    const { result: createResult } = renderHook(() => useCreateSource(), { wrapper })
    createResult.current.mutate({
      label: 'Source 2',
      kind: 'opds',
      config: { baseUrl: 'https://opds.example.com' },
    })
    await waitFor(() => expect(createResult.current.isSuccess).toBe(true))

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['sources', 'list'] })

    server.use(
      http.patch('*/api/v1/sources/:id', () =>
        HttpResponse.json({
          id: 's-1',
          label: 'Updated Source 1',
          kind: 'local-folder',
          config: { basePath: '/srv/books' },
          hasCredential: false,
          health: { status: 'reachable', checkedAt: null, detail: null },
          capabilities: { canList: true, canSearch: false, canDownload: true },
        }),
      ),
      http.delete('*/api/v1/sources/:id', () => new HttpResponse(null, { status: 204 })),
      http.post('*/api/v1/sources/:id/health-check', () =>
        HttpResponse.json({ status: 'reachable', checkedAt: null, detail: null }),
      ),
    )

    const { result: updateResult } = renderHook(() => useUpdateSource(), { wrapper })
    updateResult.current.mutate({ id: 's-1', input: { label: 'Updated Source 1' } })
    await waitFor(() => expect(updateResult.current.isSuccess).toBe(true))

    const { result: deleteResult } = renderHook(() => useDeleteSource(), { wrapper })
    deleteResult.current.mutate('s-1')
    await waitFor(() => expect(deleteResult.current.isSuccess).toBe(true))

    const { result: healthResult } = renderHook(() => useHealthCheckSource(), { wrapper })
    healthResult.current.mutate('s-1')
    await waitFor(() => expect(healthResult.current.isSuccess).toBe(true))
  })

  it('useSourceCandidates handles infinite query for browse and search', async () => {
    const { wrapper } = createTestWrapper()

    server.use(
      http.get('*/api/v1/sources/s-1/browse', () =>
        HttpResponse.json({
          items: [
            {
              title: 'Book 1',
              author: 'Author 1',
              fileReference: { referenceId: 'ref-1', format: 'EPUB', sizeBytes: 100 },
              coverUrl: null,
            },
          ],
          nextCursor: null,
        }),
      ),
    )

    const { result } = renderHook(() => useSourceCandidates('s-1'), { wrapper })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data?.pages[0]?.items).toHaveLength(1)
    expect(result.current.data?.pages[0]?.items[0]?.title).toBe('Book 1')
  })
})
