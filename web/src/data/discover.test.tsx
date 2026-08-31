import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../mocks/node'
import { fetchDiscoverSearch, fetchDiscoverWork, useDiscoverSearch, useDiscoverWork } from './discover'

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

describe('discover data hooks & fetchers (FR-1, FR-3, FR-4)', () => {
  it('fetchDiscoverSearch serializes q, limit, and offset parameters correctly', async () => {
    let capturedUrl: URL | null = null

    server.use(
      http.get('*/api/v1/discover', ({ request }) => {
        capturedUrl = new URL(request.url)
        return HttpResponse.json({ items: [], total: 0, limit: 20, offset: 0 })
      }),
    )

    const resp = await fetchDiscoverSearch({
      q: 'Middlemarch',
      limit: 10,
      offset: 20,
    })

    expect(capturedUrl).not.toBeNull()
    const url = capturedUrl! as URL
    expect(url.searchParams.get('q')).toBe('Middlemarch')
    expect(url.searchParams.get('limit')).toBe('10')
    expect(url.searchParams.get('offset')).toBe('20')
    expect(resp.total).toBe(0)
  })

  it('useDiscoverSearch stays idle and disabled when q is empty or undefined', () => {
    const { result } = renderHook(() => useDiscoverSearch({ q: '' }), {
      wrapper: createTestWrapper(),
    })

    expect(result.current.fetchStatus).toBe('idle')
    expect(result.current.data).toBeUndefined()
  })

  it('useDiscoverSearch fetches results when q is provided', async () => {
    server.use(
      http.get('*/api/v1/discover', () => {
        return HttpResponse.json({
          items: [
            {
              openLibraryWorkKey: 'OL82563W',
              title: 'Middlemarch',
              authors: [{ name: 'George Eliot' }],
              editionCount: 42,
            },
          ],
          total: 1,
          limit: 20,
          offset: 0,
        })
      }),
    )

    const { result } = renderHook(() => useDiscoverSearch({ q: 'Middlemarch' }), {
      wrapper: createTestWrapper(),
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.items).toHaveLength(1)
    expect(result.current.data?.items[0]?.title).toBe('Middlemarch')
  })

  it('fetchDiscoverWork fetches work detail correctly', async () => {
    server.use(
      http.get('*/api/v1/discover/works/OL82563W', () => {
        return HttpResponse.json({
          work: {
            title: 'Middlemarch',
            authors: [{ name: 'George Eliot' }],
          },
          editions: [],
        })
      }),
    )

    const detail = await fetchDiscoverWork('OL82563W')
    expect(detail.work.title).toBe('Middlemarch')
    expect(detail.work.authors[0]?.name).toBe('George Eliot')
  })

  it('useDiscoverWork stays disabled when openLibraryId is undefined', () => {
    const { result } = renderHook(() => useDiscoverWork(undefined), {
      wrapper: createTestWrapper(),
    })

    expect(result.current.fetchStatus).toBe('idle')
  })
})
