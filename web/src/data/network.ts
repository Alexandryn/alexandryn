import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteRequest, getJson, patchJson, postJson } from './http'

export interface InitiatePairingRequest {
  secret?: string
}

export interface InitiatePairingResponse {
  pairingId: string
  code: string
  payload: string
  address: string
  expiresAt: string
}

export interface VerifyPairingRequest {
  code: string
  label?: string
}

export interface VerifyPairingResponse {
  enrolmentGrant: string
  address: string
  hostName: string
}

export interface PairingQRResponse {
  payload: string
  address: string
  code: string
  expiresAt: string
  state: 'pending' | 'verified' | 'consumed' | 'expired'
}

export interface NetworkAddress {
  scope: string
  url: string
}

export type TLSMode = 'none' | 'static' | 'acme'
export type Reachability = 'local_only' | 'local_network' | 'internet'

export interface NetworkStatusBase {
  reachability: Reachability | string
  tlsMode: TLSMode | string
  authRequired: boolean
  address: string
}

export interface NetworkStatusAdmin extends NetworkStatusBase {
  addresses: NetworkAddress[]
  hostName: string
  acmeDomain?: string
}

/** Union type representing either reader-scoped or admin-scoped status response. */
export type NetworkStatus = NetworkStatusBase | NetworkStatusAdmin

export function isAdminNetworkStatus(status: NetworkStatus): status is NetworkStatusAdmin {
  return 'addresses' in status && Array.isArray((status as NetworkStatusAdmin).addresses)
}

export interface NetworkSettings {
  hostName: string
  rememberDeviceDays: number
  updatedAt: string
}

export interface UpdateNetworkSettingsInput {
  hostName?: string
  rememberDeviceDays?: number
}

// ── Low-level fetch functions ──────────────────────────────────────────────

export function initiatePairing(secret?: string): Promise<InitiatePairingResponse> {
  return postJson<InitiatePairingResponse>(
    '/api/v1/network/pair/initiate',
    secret ? { secret } : {},
  )
}

export function verifyPairing(data: VerifyPairingRequest): Promise<VerifyPairingResponse> {
  return postJson<VerifyPairingResponse>('/api/v1/network/pair/verify', data)
}

export function fetchPairingQR(id: string): Promise<PairingQRResponse> {
  return getJson<PairingQRResponse>(`/api/v1/network/pair/${encodeURIComponent(id)}/qr`)
}

export function fetchNetworkStatus(): Promise<NetworkStatus> {
  return getJson<NetworkStatus>('/api/v1/network/status')
}

export function updateNetworkSettings(data: UpdateNetworkSettingsInput): Promise<NetworkSettings> {
  return patchJson<NetworkSettings>('/api/v1/network/settings', data)
}

export function deletePairing(id: string): Promise<void> {
  return deleteRequest(`/api/v1/network/pair/${encodeURIComponent(id)}`)
}

// ── TanStack Query hooks (frontend-shell-and-routing.md FR-2) ───────────────

export const networkKeys = {
  all: ['network'] as const,
  status: () => [...networkKeys.all, 'status'] as const,
  pair: (id: string) => [...networkKeys.all, 'pair', id] as const,
  pairQR: (id: string) => [...networkKeys.pair(id), 'qr'] as const,
}

export function useNetworkStatus() {
  return useQuery({
    queryKey: networkKeys.status(),
    queryFn: fetchNetworkStatus,
  })
}

export function usePairingQR(id: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: networkKeys.pairQR(id),
    queryFn: () => fetchPairingQR(id),
    enabled: options?.enabled ?? Boolean(id),
  })
}

export function useInitiatePairing() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (secret?: string) => initiatePairing(secret),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: networkKeys.all })
      return data
    },
  })
}

export function useVerifyPairing() {
  return useMutation({
    mutationFn: (data: VerifyPairingRequest) => verifyPairing(data),
  })
}

export function useUpdateNetworkSettings() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: UpdateNetworkSettingsInput) => updateNetworkSettings(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: networkKeys.status() })
    },
  })
}

export function useDeletePairing() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deletePairing(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: networkKeys.all })
    },
  })
}
