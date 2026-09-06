import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteRequest, getJson } from './http'

export interface DeviceItem {
  id: string
  label: string
  deviceClass: string
  enrolledVia: string
  createdAt: string
  lastSeenAt: string
  lastSyncedAt?: string | null
  revokedAt?: string | null
}

export interface ListDevicesResponse {
  devices: DeviceItem[]
}

export function fetchDevices(): Promise<ListDevicesResponse> {
  return getJson<ListDevicesResponse>('/api/v1/devices')
}

export function revokeDevice(id: string): Promise<void> {
  return deleteRequest(`/api/v1/devices/${id}`)
}

export function useDevices() {
  return useQuery({
    queryKey: ['devices'],
    queryFn: fetchDevices,
    staleTime: 0,
  })
}

export function useRevokeDevice() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => revokeDevice(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['devices'] })
    },
  })
}
