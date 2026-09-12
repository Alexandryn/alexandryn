import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import {
  deletePairing,
  fetchNetworkStatus,
  fetchPairingQR,
  initiatePairing,
  isAdminNetworkStatus,
  updateNetworkSettings,
  useDeletePairing,
  useInitiatePairing,
  useNetworkStatus,
  usePairingQR,
  useUpdateNetworkSettings,
  useVerifyPairing,
  verifyPairing,
} from './network'
import { login } from './auth'

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

describe('network data layer', () => {
  it('fetchNetworkStatus fetches status and isAdminNetworkStatus discriminates correctly', async () => {
    const status = await fetchNetworkStatus()
    expect(status).toBeDefined()
    expect(status.reachability).toBeDefined()
    expect(status.tlsMode).toBeDefined()
    expect(status.authRequired).toBe(true)

    // With MSW fixture, getNetworkStatus is admin status with addresses array
    expect(isAdminNetworkStatus(status)).toBe(true)
    if (isAdminNetworkStatus(status)) {
      expect(Array.isArray(status.addresses)).toBe(true)
      expect(status.hostName).toBeDefined()
    }
  })

  it('useNetworkStatus hook returns status', async () => {
    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useNetworkStatus(), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data).toBeDefined()
    expect(result.current.data?.authRequired).toBe(true)
  })

  it('initiatePairing and useInitiatePairing create a pairing session', async () => {
    const resp = await initiatePairing()
    expect(resp.pairingId).toBeDefined()
    expect(resp.code).toBeDefined()
    expect(resp.payload).toContain('/connect?c=')

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useInitiatePairing(), { wrapper })

    result.current.mutate('optional-secret')
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.code).toBeDefined()
  })

  it('verifyPairing and useVerifyPairing verify valid code and reject bad code', async () => {
    const resp = await verifyPairing({ code: 'ABCD-EFGH', label: 'Test iPad' })
    expect(resp.enrolmentGrant).toBeDefined()
    expect(resp.address).toBeDefined()

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useVerifyPairing(), { wrapper })

    result.current.mutate({ code: 'ABCD-EFGH' })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.enrolmentGrant).toBeDefined()

    // Test rejection on bad code
    await expect(verifyPairing({ code: '0000-0000' })).rejects.toThrow()
  })

  it('fetchPairingQR and usePairingQR retrieve payload and code', async () => {
    const resp = await fetchPairingQR('sess-1')
    expect(resp.payload).toBeDefined()
    expect(resp.code).toBeDefined()

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => usePairingQR('sess-1'), { wrapper })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.payload).toBeDefined()
  })

  it('updateNetworkSettings and useUpdateNetworkSettings update hostName and rememberDeviceDays', async () => {
    const updated = await updateNetworkSettings({
      hostName: 'my-library.local',
      rememberDeviceDays: 14,
    })
    expect(updated.hostName).toBe('my-library.local')
    expect(updated.rememberDeviceDays).toBe(14)

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useUpdateNetworkSettings(), { wrapper })

    result.current.mutate({ rememberDeviceDays: 7 })
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
    expect(result.current.data?.rememberDeviceDays).toBe(7)
  })

  it('deletePairing and useDeletePairing remove pairing session', async () => {
    await expect(deletePairing('sess-1')).resolves.toBeUndefined()

    const { wrapper } = createTestWrapper()
    const { result } = renderHook(() => useDeletePairing(), { wrapper })

    result.current.mutate('sess-1')
    await waitFor(() => expect(result.current.isSuccess).toBe(true))
  })

  it('login accepts optional enrolmentGrant parameter', async () => {
    // Should succeed with enrolmentGrant passed
    const res = await login({
      emailOrUsername: 'testadmin',
      password: 'password123',
      enrolmentGrant: 'enrol-grant-token-12345',
    })
    expect(res).toBeDefined()
  })
})
