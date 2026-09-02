import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { server } from '../mocks/node'
import {
  downloadReadingExport,
  readerDeviceId,
  useReadingPreferences,
  useReportProgress,
} from './reading'

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

beforeEach(() => {
  window.localStorage.clear()
})

describe('reading data layer (frontend-reader.md)', () => {
  it('generates and persists a stable device id (FR-8)', () => {
    const first = readerDeviceId()
    expect(first).toMatch(/^[0-9a-f-]{36}$/i)
    expect(readerDeviceId()).toBe(first)
    expect(window.localStorage.getItem('alexandryn.reader.deviceId')).toBe(first)
  })

  it('sends X-Device-Id on a progress report', async () => {
    let header: string | null = null
    server.use(
      http.post('*/api/v1/reading/works/:workId/progress', ({ request }) => {
        header = request.headers.get('X-Device-Id')
        return HttpResponse.json({
          progress: { percentage: 0.5, epoch: 0, precisePosition: null, observedAt: 'x' },
          outcome: 'advanced',
        })
      }),
    )

    const { result } = renderHook(() => useReportProgress('work-1'), { wrapper: wrapper() })
    result.current.mutate({ percentage: 0.5, observedEpoch: 0 })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(header).toMatch(/^[0-9a-f-]{36}$/i)
  })

  it('reads preferences with the device header', async () => {
    let header: string | null = null
    server.use(
      http.get('*/api/v1/reading/preferences', ({ request }) => {
        header = request.headers.get('X-Device-Id')
        return HttpResponse.json({
          preferences: {
            font: 'serif',
            fontSize: 19,
            lineSpacing: 1.5,
            theme: 'light',
            layoutMode: 'paginated',
            columnWidth: 'default',
          },
        })
      }),
    )

    const { result } = renderHook(() => useReadingPreferences(), { wrapper: wrapper() })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(header).toBeTruthy()
    expect(result.current.data?.preferences.theme).toBe('light')
  })

  it('downloads the export as a JSON file', async () => {
    server.use(
      http.get('*/api/v1/reading/export', () =>
        HttpResponse.json({ schemaVersion: 1, exportedAt: 'x', works: [], editions: [] }),
      ),
    )
    let clicked = false
    const realCreate = document.createElement.bind(document)
    const spy = vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = realCreate(tag)
      if (tag === 'a') el.click = () => (clicked = true)
      return el
    })

    await downloadReadingExport()
    expect(clicked).toBe(true)
    spy.mockRestore()
  })
})
