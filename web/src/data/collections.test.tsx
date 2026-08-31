import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import { server } from '../mocks/node'
import {
  addWorkToCollection,
  createCollection,
  deleteCollection,
  fetchCollection,
  fetchCollections,
  removeWorkFromCollection,
  renameCollection,
  useCollections,
  useCreateCollection,
} from './collections'


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

describe('collections data layer (FR-1 to FR-7)', () => {
  it('fetchCollections fetches list of collection summaries', async () => {
    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({
          collections: [
            { id: 'c-1', name: 'Classics', workCount: 5 },
            { id: 'c-2', name: 'Sci-Fi', workCount: 2 },
          ],
        }),
      ),
    )

    const collections = await fetchCollections()
    expect(collections).toHaveLength(2)
    expect(collections[0]?.name).toBe('Classics')
    expect(collections[0]?.workCount).toBe(5)
  })


  it('fetchCollection fetches collection detail with member works', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', ({ params }) => {
        expect(params.id).toBe('c-1')
        return HttpResponse.json({
          id: 'c-1',
          name: 'Classics',
          works: [
            {
              id: 'w-1',
              title: 'Middlemarch',
              subtitle: '',
              authors: ['George Eliot'],
              isOwned: true,
              collections: [{ id: 'c-1', name: 'Classics' }],
              addedAt: '2026-01-15T10:00:00Z',
            },
          ],
        })
      }),
    )

    const detail = await fetchCollection('c-1')
    expect(detail.id).toBe('c-1')
    expect(detail.name).toBe('Classics')
    expect(detail.works).toHaveLength(1)
  })

  it('createCollection sends POST with name', async () => {
    let capturedBody: unknown = null
    server.use(
      http.post('*/api/v1/collections', async ({ request }) => {
        capturedBody = await request.json()
        return HttpResponse.json(
          { id: 'c-new', name: 'New Coll', works: [] },
          { status: 201 },
        )
      }),
    )

    const created = await createCollection('New Coll')
    expect(capturedBody).toEqual({ name: 'New Coll' })
    expect(created.id).toBe('c-new')
  })

  it('renameCollection sends PATCH with new name', async () => {
    let capturedBody: unknown = null
    server.use(
      http.patch('*/api/v1/collections/:id', async ({ request, params }) => {
        expect(params.id).toBe('c-1')
        capturedBody = await request.json()
        return HttpResponse.json({ id: 'c-1', name: 'Renamed Coll', works: [] })
      }),
    )

    const renamed = await renameCollection('c-1', 'Renamed Coll')
    expect(capturedBody).toEqual({ name: 'Renamed Coll' })
    expect(renamed.name).toBe('Renamed Coll')
  })

  it('deleteCollection sends DELETE request', async () => {
    let deletedId: string | undefined
    server.use(
      http.delete('*/api/v1/collections/:id', ({ params }) => {
        deletedId = params.id as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    await deleteCollection('c-1')
    expect(deletedId).toBe('c-1')
  })

  it('addWorkToCollection sends POST with workId', async () => {
    let capturedBody: unknown = null
    server.use(
      http.post('*/api/v1/collections/:id/works', async ({ request, params }) => {
        expect(params.id).toBe('c-1')
        capturedBody = await request.json()
        return HttpResponse.json({ id: 'c-1', name: 'Classics', works: [] })
      }),
    )

    await addWorkToCollection('c-1', 'w-1')
    expect(capturedBody).toEqual({ workId: 'w-1' })
  })

  it('removeWorkFromCollection sends DELETE request', async () => {
    let capturedCollectionId: string | undefined
    let capturedWorkId: string | undefined
    server.use(
      http.delete('*/api/v1/collections/:id/works/:workId', ({ params }) => {
        capturedCollectionId = params.id as string
        capturedWorkId = params.workId as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    await removeWorkFromCollection('c-1', 'w-1')
    expect(capturedCollectionId).toBe('c-1')
    expect(capturedWorkId).toBe('w-1')
  })

  it('useCollections and mutation hooks invalidate query caches', async () => {
    const { queryClient, wrapper } = createTestWrapper()
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    server.use(
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({
          collections: [{ id: 'c-1', name: 'Classics', workCount: 0 }],
        }),
      ),
      http.post('*/api/v1/collections', () =>
        HttpResponse.json(
          { id: 'c-2', name: 'Fiction', works: [] },
          { status: 201 },
        ),
      ),
    )

    const { result: listResult } = renderHook(() => useCollections(), { wrapper })
    await waitFor(() => expect(listResult.current.isSuccess).toBe(true))

    const { result: createResult } = renderHook(() => useCreateCollection(), { wrapper })

    createResult.current.mutate('Fiction')
    await waitFor(() => expect(createResult.current.isSuccess).toBe(true))

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['collections'] })
  })
})

