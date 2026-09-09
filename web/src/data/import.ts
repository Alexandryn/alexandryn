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
  refetchInterval?: number | false
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
    refetchInterval,
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
