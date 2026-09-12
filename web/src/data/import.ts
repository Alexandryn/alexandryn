import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getJson, postJson } from './http'

export type ImportCandidateStatus =
  | 'queued'
  | 'pending'
  | 'auto_imported'
  | 'confirmed'
  | 'rejected'
  | 'failed'

export interface ImportCandidateFileRef {
  id: string
  format: string
  sizeBytes?: number | null
}

export interface ExtractedMetadata {
  title?: string
  authors?: string[]
  isbn?: string | null
  language?: string | null
  publisher?: string | null
  description?: string | null
  coverBytes?: string | null // base64 encoded
  format?: string
}

export interface MatchCandidate {
  type: string
  confidence: 'exact' | 'high' | 'medium' | 'low'
  title?: string
  author?: string
  coverUrl?: string | null
  editionId?: string
  openLibraryWorkKey?: string
}

export interface ImportCandidate {
  id: string
  sourceId: string
  fileReference: ImportCandidateFileRef
  status: ImportCandidateStatus
  extractedMetadata?: ExtractedMetadata | null
  matchCandidates?: MatchCandidate[] | null
  jobId?: string | null
  lastError?: string | null
  createdAt: string
  updatedAt: string
}

export interface ImportDiscoverResponse {
  discoveredCount: number
  skippedCount: number
  jobIds: string[]
}

export interface ImportConfirmRequest {
  action: 'attach_existing' | 'use_open_library_match' | 'create_new'
  editionId?: string
  openLibraryWorkKey?: string
  metadata?: Partial<ExtractedMetadata>
}

export interface UseImportCandidatesOptions {
  sourceId?: string
  status?: ImportCandidateStatus
  /**
   * `true` turns on adaptive polling: fast (2s) while there is at least
   * one candidate in this status, a slow floor (20s) when the list is
   * empty, and stopped entirely while the tab is hidden. A number/false
   * is still honoured for a fixed cadence.
   */
  refetchInterval?: number | false | true
}

const IMPORT_POLL_FAST_MS = 2000
const IMPORT_POLL_IDLE_MS = 20000

/** Adaptive import-poll cadence — see UseImportCandidatesOptions. */
export function importPollInterval(candidateCount: number, hidden: boolean): number | false {
  if (hidden) return false
  return candidateCount > 0 ? IMPORT_POLL_FAST_MS : IMPORT_POLL_IDLE_MS
}

export function useImportCandidates(options: UseImportCandidatesOptions = {}) {
  const { sourceId, status, refetchInterval } = options
  const params = new URLSearchParams()
  if (sourceId) params.set('sourceId', sourceId)
  if (status) params.set('status', status)

  const queryKey = ['import-candidates', { sourceId, status }]
  const url = `/api/v1/import/candidates${params.toString() ? `?${params.toString()}` : ''}`

  return useQuery<{ candidates: ImportCandidate[] }>({
    queryKey,
    queryFn: ({ signal }) => getJson<{ candidates: ImportCandidate[] }>(url, {}, { signal }),
    refetchInterval:
      refetchInterval === true
        ? (query) =>
            importPollInterval(
              query.state.data?.candidates?.length ?? 0,
              typeof document !== 'undefined' && document.visibilityState === 'hidden',
            )
        : refetchInterval,
    refetchOnWindowFocus: refetchInterval === true ? true : undefined,
  })
}

export function useDiscoverImport() {
  const queryClient = useQueryClient()
  return useMutation<ImportDiscoverResponse, Error, { sourceId: string }>({
    mutationFn: ({ sourceId }) =>
      postJson<ImportDiscoverResponse>('/api/v1/import/discover', { sourceId }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['import-candidates'] })
    },
  })
}

export function useConfirmImportCandidate() {
  const queryClient = useQueryClient()
  return useMutation<ImportCandidate, Error, { id: string; payload: ImportConfirmRequest }>({
    mutationFn: ({ id, payload }) =>
      postJson<ImportCandidate>(`/api/v1/import/candidates/${encodeURIComponent(id)}/confirm`, payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['import-candidates'] })
      void queryClient.invalidateQueries({ queryKey: ['library'] })
    },
  })
}

export function useRejectImportCandidate() {
  const queryClient = useQueryClient()
  return useMutation<ImportCandidate, Error, string>({
    mutationFn: (id) =>
      postJson<ImportCandidate>(`/api/v1/import/candidates/${encodeURIComponent(id)}/reject`),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['import-candidates'] })
    },
  })
}
