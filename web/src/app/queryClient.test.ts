import { QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { useQuery } from '@tanstack/react-query'
import { createElement, type ReactNode } from 'react'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { server } from '../mocks/node'
import { getJson } from '../data/http'
import { makeQueryClient } from './queryClient'

function wrap(client = makeQueryClient()) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client }, children)
}

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('makeQueryClient', () => {
  it('does not retry a 4xx response', async () => {
    let calls = 0
    server.use(
      http.get('*/api/v1/thing', () => {
        calls++
        return HttpResponse.json({ code: 'NotFound', message: 'no' }, { status: 404 })
      }),
    )
    const { result } = renderHook(() => useQuery({ queryKey: ['t'], queryFn: () => getJson('/api/v1/thing') }), {
      wrapper: wrap(),
    })
    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(calls).toBe(1)
  })

  it('on a 401, tries a single refresh then clears session and redirects to /login', async () => {
    window.localStorage.setItem('alexandryn_access_token', 'expired')
    window.localStorage.setItem('alexandryn_refresh_token', 'rt-1')
    const assign = vi.fn()
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      pathname: '/library',
      search: '',
      assign,
    } as unknown as Location)

    let refreshCalls = 0
    server.use(
      http.get('*/api/v1/thing', () =>
        HttpResponse.json({ code: 'Unauthorized', message: 'expired' }, { status: 401 }),
      ),
      http.post('*/api/v1/auth/refresh', () => {
        refreshCalls++
        return HttpResponse.json({ code: 'Unauthorized', message: 'no' }, { status: 401 })
      }),
    )

    const { result } = renderHook(
      () => useQuery({ queryKey: ['t2'], queryFn: () => getJson('/api/v1/thing'), retry: false }),
      { wrapper: wrap() },
    )
    await waitFor(() => expect(result.current.isError).toBe(true))
    await waitFor(() => expect(assign).toHaveBeenCalled())

    expect(refreshCalls).toBe(1)
    expect(window.localStorage.getItem('alexandryn_access_token')).toBeNull()
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/login?next=%2Flibrary'))
  })

  it('redirects to login when a refresh returns 200 without an access token (no refresh loop)', async () => {
    window.localStorage.setItem('alexandryn_access_token', 'expired')
    window.localStorage.setItem('alexandryn_refresh_token', 'rt-1')
    const assign = vi.fn()
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      pathname: '/library',
      search: '',
      assign,
    } as unknown as Location)

    let refreshCalls = 0
    let thingCalls = 0
    server.use(
      http.get('*/api/v1/thing', () => {
        thingCalls++
        return HttpResponse.json({ code: 'Unauthorized', message: 'expired' }, { status: 401 })
      }),
      http.post('*/api/v1/auth/refresh', () => {
        refreshCalls++
        // 200, but no accessToken — a misbehaving server.
        return HttpResponse.json({ user: { id: 'u1' } })
      }),
    )

    renderHook(
      () => useQuery({ queryKey: ['t3'], queryFn: () => getJson('/api/v1/thing'), retry: false }),
      { wrapper: wrap() },
    )
    await waitFor(() => expect(assign).toHaveBeenCalled())

    // One refresh attempt, then straight to login — never re-entered.
    expect(refreshCalls).toBe(1)
    expect(thingCalls).toBe(1)
    expect(assign).toHaveBeenCalledWith(expect.stringContaining('/login?next=%2Flibrary'))
  })

  it('bounds refresh+refetch cycles when a fresh token is still rejected', async () => {
    window.localStorage.setItem('alexandryn_access_token', 'expired')
    window.localStorage.setItem('alexandryn_refresh_token', 'rt-1')
    const assign = vi.fn()
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      pathname: '/library',
      search: '',
      assign,
    } as unknown as Location)

    let refreshCalls = 0
    server.use(
      http.get('*/api/v1/thing', () =>
        HttpResponse.json({ code: 'Unauthorized', message: 'expired' }, { status: 401 }),
      ),
      http.post('*/api/v1/auth/refresh', () => {
        refreshCalls++
        // Server keeps minting tokens the API still rejects.
        return HttpResponse.json({ accessToken: `tok-${refreshCalls}` })
      }),
    )

    renderHook(
      () => useQuery({ queryKey: ['t4'], queryFn: () => getJson('/api/v1/thing'), retry: false }),
      { wrapper: wrap() },
    )
    await waitFor(() => expect(assign).toHaveBeenCalled())

    // Bounded, not unbounded: a small number of attempts then login.
    expect(refreshCalls).toBeLessThanOrEqual(3)
  })
})
