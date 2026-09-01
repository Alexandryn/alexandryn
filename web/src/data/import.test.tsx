import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { server } from '../mocks/node'
import {
  useConfirmImportCandidate,
  useDiscoverImport,
  useImportCandidates,
  useRejectImportCandidate,
} from './import'

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

describe('Import Data Hooks', () => {
  it('useImportCandidates fetches candidate list', async () => {
    server.use(
      http.get('*/api/v1/import/candidates', () =>
        HttpResponse.json({
          candidates: [
            {
              id: 'cand-1',
              sourceId: 'src-1',
              fileReference: { id: 'test.epub', format: 'epub' },
              status: 'pending',
              extractedMetadata: { title: 'Test Book' },
              createdAt: '2026-09-01T12:00:00Z',
              updatedAt: '2026-09-01T12:00:00Z',
            },
          ],
        }),
      ),
    )

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useImportCandidates(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.candidates).toHaveLength(1)
    expect(result.current.data?.candidates?.[0]?.id).toBe('cand-1')
  })

  it('useDiscoverImport mutates and invalidates cache', async () => {
    let capturedBody: unknown = null
    server.use(
      http.post('*/api/v1/import/discover', async ({ request }) => {
        capturedBody = await request.json()
        return HttpResponse.json(
          { discoveredCount: 3, skippedCount: 1, jobIds: ['job-1'] },
          { status: 202 },
        )
      }),
    )

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useDiscoverImport(), { wrapper })

    result.current.mutate({ sourceId: 'src-1' })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(capturedBody).toEqual({ sourceId: 'src-1' })
    expect(result.current.data?.discoveredCount).toBe(3)
  })

  it('useConfirmImportCandidate sends confirm payload', async () => {
    let capturedPath = ''
    let capturedBody: unknown = null
    server.use(
      http.post('*/api/v1/import/candidates/:id/confirm', async ({ params, request }) => {
        capturedPath = String(params.id)
        capturedBody = await request.json()
        return HttpResponse.json({
          id: 'cand-1',
          sourceId: 'src-1',
          fileReference: { id: 'test.epub', format: 'epub' },
          status: 'confirmed',
          createdAt: '2026-09-01T12:00:00Z',
          updatedAt: '2026-09-01T12:00:00Z',
        })
      }),
    )

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useConfirmImportCandidate(), { wrapper })

    result.current.mutate({
      id: 'cand-1',
      payload: { action: 'attach_existing', editionId: 'ed-1' },
    })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(capturedPath).toBe('cand-1')
    expect(capturedBody).toEqual({ action: 'attach_existing', editionId: 'ed-1' })
    expect(result.current.data?.status).toBe('confirmed')
  })

  it('useRejectImportCandidate sends reject request', async () => {
    let capturedPath = ''
    server.use(
      http.post('*/api/v1/import/candidates/:id/reject', ({ params }) => {
        capturedPath = String(params.id)
        return HttpResponse.json({
          id: 'cand-1',
          sourceId: 'src-1',
          fileReference: { id: 'test.epub', format: 'epub' },
          status: 'rejected',
          createdAt: '2026-09-01T12:00:00Z',
          updatedAt: '2026-09-01T12:00:00Z',
        })
      }),
    )

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useRejectImportCandidate(), { wrapper })

    result.current.mutate('cand-1')

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(capturedPath).toBe('cand-1')
    expect(result.current.data?.status).toBe('rejected')
  })
})
