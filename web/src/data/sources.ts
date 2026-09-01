import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteRequest, getJson, patchJson, postJson } from './http'

export type SourceKind = 'local-folder' | 'opds'

export type SourceHealthStatus = 'unknown' | 'reachable' | 'unreachable'

export type SourceHealthDetail =
  | 'timeout'
  | 'connection-refused'
  | 'auth-rejected'
  | 'http-4xx'
  | 'http-5xx'
  | 'http-3xx-unsupported'
  | 'unparseable-response'
  | 'path-not-found'
  | 'path-not-readable'
  | null

export interface SourceHealth {
  status: SourceHealthStatus
  checkedAt: string | null
  detail: SourceHealthDetail
}

export interface SourceCapabilities {
  canList: boolean
  canSearch: boolean
  canDownload: boolean
}

export interface SourceConfig {
  basePath?: string
  baseUrl?: string
}

export interface Source {
  id: string
  label: string
  kind: SourceKind
  config: SourceConfig
  hasCredential: boolean
  health: SourceHealth
  capabilities: SourceCapabilities
}

export interface SourceCredentialInput {
  username: string
  password: string
}

export interface SourceCreate {
  label: string
  kind: SourceKind
  config: SourceConfig
  credential?: SourceCredentialInput
}

export interface SourceUpdate {
  label?: string
  config?: SourceConfig
  credential?: SourceCredentialInput
}

export interface FileReference {
  referenceId: string
  format: string
  sizeBytes: number | null
}

export interface SourceCandidate {
  title: string
  author: string | null
  fileReference: FileReference
  coverUrl: string | null
}

export interface SourceCandidatePage {
  items: SourceCandidate[]
  nextCursor: string | null
}

export const sourceKeys = {
  all: ['sources'] as const,
  lists: () => [...sourceKeys.all, 'list'] as const,
  detail: (id: string) => [...sourceKeys.all, 'detail', id] as const,
  candidates: (id: string, params?: { query?: string }) =>
    [...sourceKeys.all, 'candidates', id, params] as const,
}

export async function fetchSources(): Promise<Source[]> {
  const data = await getJson<{ sources: Source[] }>('/api/v1/sources')
  return data.sources
}

export function fetchSource(id: string): Promise<Source> {
  return getJson<Source>(`/api/v1/sources/${encodeURIComponent(id)}`)
}

export function createSource(input: SourceCreate): Promise<Source> {
  return postJson<Source>('/api/v1/sources', input)
}

export function updateSource(id: string, input: SourceUpdate): Promise<Source> {
  return patchJson<Source>(`/api/v1/sources/${encodeURIComponent(id)}`, input)
}

export function deleteSource(id: string): Promise<void> {
  return deleteRequest(`/api/v1/sources/${encodeURIComponent(id)}`)
}

export function checkSourceHealth(id: string): Promise<SourceHealth> {
  return postJson<SourceHealth>(`/api/v1/sources/${encodeURIComponent(id)}/health-check`)
}

export function fetchSourceBrowse(
  id: string,
  params: { limit?: number; cursor?: string } = {},
): Promise<SourceCandidatePage> {
  const search = new URLSearchParams()
  if (params.limit && params.limit > 0) {
    search.set('limit', String(params.limit))
  }
  if (params.cursor) {
    search.set('cursor', params.cursor)
  }
  const qs = search.toString()
  return getJson<SourceCandidatePage>(
    `/api/v1/sources/${encodeURIComponent(id)}/browse${qs ? `?${qs}` : ''}`,
  )
}

export function fetchSourceSearch(
  id: string,
  params: { q: string; limit?: number; cursor?: string },
): Promise<SourceCandidatePage> {
  const search = new URLSearchParams()
  search.set('q', params.q.trim())
  if (params.limit && params.limit > 0) {
    search.set('limit', String(params.limit))
  }
  if (params.cursor) {
    search.set('cursor', params.cursor)
  }
  const qs = search.toString()
  return getJson<SourceCandidatePage>(
    `/api/v1/sources/${encodeURIComponent(id)}/search${qs ? `?${qs}` : ''}`,
  )
}

/**
 * Sources list hook using TanStack Query useQuery.
 * Cache key: ['sources', 'list'].
 */
export function useSources() {
  return useQuery({
    queryKey: sourceKeys.lists(),
    queryFn: fetchSources,
  })
}

/**
 * Single source detail hook using TanStack Query useQuery.
 * Cache key: ['sources', 'detail', id].
 */
export function useSource(id: string | undefined) {
  return useQuery({
    queryKey: id ? sourceKeys.detail(id) : sourceKeys.all,
    queryFn: () => (id ? fetchSource(id) : Promise.reject(new Error('id is required'))),
    enabled: Boolean(id && id.trim() !== ''),
  })
}

/**
 * Mutation for creating a new source.
 * Invalidates ['sources', 'list'] on success.
 */
export function useCreateSource() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: SourceCreate) => createSource(input),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: sourceKeys.lists() })
    },
  })
}

/**
 * Mutation for updating a source.
 * Invalidates ['sources', 'list'] and ['sources', 'detail', id] on success.
 */
export function useUpdateSource() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: SourceUpdate }) => updateSource(id, input),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: sourceKeys.lists() })
      void queryClient.invalidateQueries({ queryKey: sourceKeys.detail(data.id) })
    },
  })
}

/**
 * Mutation for deleting a source.
 * Invalidates ['sources', 'list'] and removes ['sources', 'detail', id] on success.
 */
export function useDeleteSource() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteSource(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({ queryKey: sourceKeys.lists() })
      queryClient.removeQueries({ queryKey: sourceKeys.detail(id) })
    },
  })
}

/**
 * Mutation for running a health check on a source.
 * Invalidates ['sources', 'list'] and ['sources', 'detail', id] on success.
 */
export function useHealthCheckSource() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => checkSourceHealth(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({ queryKey: sourceKeys.lists() })
      void queryClient.invalidateQueries({ queryKey: sourceKeys.detail(id) })
    },
  })
}

/**
 * Cursor-paginated source candidates hook using TanStack Query useInfiniteQuery.
 * Automatically delegates to search or browse based on query presence.
 * Cache key: ['sources', 'candidates', sourceId, { query }].
 */
export function useSourceCandidates(
  sourceId: string | undefined,
  params: { query?: string; limit?: number } = {},
) {
  const q = params.query?.trim() || undefined
  const limit = params.limit

  return useInfiniteQuery({
    queryKey: sourceId ? sourceKeys.candidates(sourceId, { query: q }) : sourceKeys.all,
    queryFn: ({ pageParam }) => {
      if (!sourceId) {
        return Promise.reject(new Error('sourceId is required'))
      }
      if (q) {
        return fetchSourceSearch(sourceId, { q, limit, cursor: pageParam })
      }
      return fetchSourceBrowse(sourceId, { limit, cursor: pageParam })
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
    enabled: Boolean(sourceId && sourceId.trim() !== ''),
  })
}
