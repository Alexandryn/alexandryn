import { useQuery } from '@tanstack/react-query'
import { getJson } from './http'

export interface DiagnosticsVersion {
  commit: string
  build_time: string
}

export interface DiagnosticsRuntime {
  goroutines: number
  heap_alloc_bytes: number
  total_alloc_bytes: number
  gc_cycles: number
}

export interface DiagnosticsResponse {
  uptime_seconds: number
  version: DiagnosticsVersion
  runtime: DiagnosticsRuntime
  metrics?: Record<string, unknown>
}

export function fetchDiagnostics(): Promise<DiagnosticsResponse> {
  return getJson<DiagnosticsResponse>('/api/v1/diagnostics')
}

export function useDiagnostics() {
  return useQuery({
    queryKey: ['diagnostics'],
    queryFn: fetchDiagnostics,
    refetchInterval: 5000,
  })
}
