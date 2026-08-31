import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../mocks/node'
import { fetchLibraryPage, fetchWorkDetail, useLibrary, useWork } from './library'

function createTestWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return function TestWrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}

describe('library data hooks & fetchers (FR-1, FR-3, FR-5)', () => {
  it('fetchLibraryPage serializes query, filter, sort, and cursor params correctly', async () => {
    let capturedUrl: URL | null = null

    server.use(
      http.get('*/api/v1/library', ({ request }) => {
        capturedUrl = new URL(request.url)
        return HttpResponse.json({ works: [], nextCursor: 'next-123' })
      }),
    )

    const page = await fetchLibraryPage({
      q: 'George Eliot',
      filter: 'owned',
      sort: 'title',
      cursor: 'cursor-abc',
      limit: 25,
    })

    expect(capturedUrl).not.toBeNull()
    if (!capturedUrl) throw new Error('capturedUrl is null')
    const url = capturedUrl as URL
    expect(url.searchParams.get('q')).toBe('George Eliot')
    expect(url.searchParams.get('filter')).toBe('owned')
    expect(url.searchParams.get('sort')).toBe('title')
    expect(url.searchParams.get('cursor')).toBe('cursor-abc')
    expect(url.searchParams.get('limit')).toBe('25')
    expect(page.nextCursor).toBe('next-123')
  })

  it('fetchLibraryPage omits default filter and sort params', async () => {
    let capturedUrl: URL | null = null

    server.use(
      http.get('*/api/v1/library', ({ request }) => {
        capturedUrl = new URL(request.url)
        return HttpResponse.json({ works: [], nextCursor: null })
      }),
    )

    await fetchLibraryPage({
      filter: 'all',
      sort: 'added_at',
    })

    expect(capturedUrl).not.toBeNull()
    if (!capturedUrl) throw new Error('capturedUrl is null')
    const url = capturedUrl as URL
    expect(url.searchParams.has('filter')).toBe(false)
    expect(url.searchParams.has('sort')).toBe(false)
  })

  it('fetchWorkDetail fetches a single work by id', async () => {
    const mockDetail = {
      id: 'work-123',
      title: 'Silas Marner',
      subtitle: '',
      authors: ['George Eliot'],
      subjects: ['Fiction'],
      originalLanguage: 'en',
      ownedEditions: [],
      collections: [],
    }

    server.use(
      http.get('*/api/v1/works/:id', ({ params }) => {
        expect(params.id).toBe('work-123')
        return HttpResponse.json(mockDetail)
      }),
    )

    const detail = await fetchWorkDetail('work-123')
    expect(detail.id).toBe('work-123')
    expect(detail.title).toBe('Silas Marner')
  })

  it('useLibrary infinite query manages pagination state', async () => {
    server.use(
      http.get('*/api/v1/library', ({ request }) => {
        const url = new URL(request.url)
        const cursor = url.searchParams.get('cursor')
        if (!cursor) {
          return HttpResponse.json({
            works: [
              {
                id: 'w-1',
                title: 'Book 1',
                subtitle: '',
                authors: ['A1'],
                isOwned: true,
                collections: [],
                addedAt: '2026-01-01T00:00:00Z',
              },
            ],
            nextCursor: 'cur-2',
          })
        }
        return HttpResponse.json({
          works: [
            {
              id: 'w-2',
              title: 'Book 2',
              subtitle: '',
              authors: ['A2'],
              isOwned: true,
              collections: [],
              addedAt: '2026-01-02T00:00:00Z',
            },
          ],
          nextCursor: null,
        })
      }),
    )

    const { result } = renderHook(() => useLibrary({ filter: 'all', sort: 'added_at' }), {
      wrapper: createTestWrapper(),
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.pages[0]?.works).toHaveLength(1)
    expect(result.current.hasNextPage).toBe(true)

    result.current.fetchNextPage()

    await waitFor(() => expect(result.current.data?.pages).toHaveLength(2))
    expect(result.current.hasNextPage).toBe(false)
  })

  it('useWork hook is disabled when id is empty or undefined', () => {
    const { result } = renderHook(() => useWork(''), {
      wrapper: createTestWrapper(),
    })

    expect(result.current.fetchStatus).toBe('idle')
    expect(result.current.data).toBeUndefined()
  })
})
