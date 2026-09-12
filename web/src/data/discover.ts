import { useQuery } from '@tanstack/react-query'
import { getJson } from './http'

export interface NormalisedAuthor {
  openLibraryAuthorKey?: string | null
  name: string
}

export interface NormalisedSearchResult {
  openLibraryWorkKey: string
  title: string
  authors: NormalisedAuthor[]
  firstPublishYear?: number | null
  coverUrl?: string | null
  editionCount: number
}

export interface NormalisedSearchResponse {
  items: NormalisedSearchResult[]
  total: number
  limit: number
  offset: number
}

export interface NormalisedWork {
  title: string
  subtitle?: string | null
  description?: string | null
  subjects?: string[]
  authors: NormalisedAuthor[]
  coverUrl?: string | null
}

export interface NormalisedEdition {
  title: string
  publisher?: string | null
  publishDate?: string | null
  language?: string | null
  openLibraryEditionKey: string
  coverUrl?: string | null
}

export interface DiscoverWorkDetail {
  work: NormalisedWork
  editions: NormalisedEdition[]
}

export interface DiscoverSearchParams {
  q: string
  limit?: number
  offset?: number
}

export function fetchDiscoverSearch(
  params: DiscoverSearchParams,
  signal?: AbortSignal,
): Promise<NormalisedSearchResponse> {
  const search = new URLSearchParams()
  search.set('q', params.q.trim())
  if (params.limit !== undefined && params.limit > 0) {
    search.set('limit', String(params.limit))
  }
  if (params.offset !== undefined && params.offset >= 0) {
    search.set('offset', String(params.offset))
  }

  const qs = search.toString()
  const path = `/api/v1/discover${qs ? `?${qs}` : ''}`
  return getJson<NormalisedSearchResponse>(path, {}, { signal })
}

/**
 * Standard paginated query for Open Library discover search.
 * Query key follows ['discover', 'search', { q, limit, offset }] cache convention.
 */
export function useDiscoverSearch(params: {
  q?: string
  limit?: number
  offset?: number
}) {
  const q = params.q?.trim() || undefined
  const limit = params.limit ?? 20
  const offset = params.offset ?? 0

  return useQuery({
    queryKey: ['discover', 'search', { q, limit, offset }],
    queryFn: ({ signal }) => {
      if (!q) {
        return Promise.reject(new Error('q is required'))
      }
      // A superseded search (the user kept typing) or an unmount aborts
      // the in-flight request rather than letting it complete unseen.
      return fetchDiscoverSearch({ q, limit, offset }, signal)
    },
    enabled: Boolean(q && q !== ''),
  })
}

export function fetchDiscoverWork(
  openLibraryId: string,
  signal?: AbortSignal,
): Promise<DiscoverWorkDetail> {
  return getJson<DiscoverWorkDetail>(
    `/api/v1/discover/works/${encodeURIComponent(openLibraryId)}`,
    {},
    { signal },
  )
}

/**
 * Single Open Library work detail hook.
 * Query key follows ['discover', 'work', openLibraryId] cache convention.
 */
export function useDiscoverWork(openLibraryId: string | undefined) {
  return useQuery({
    queryKey: ['discover', 'work', openLibraryId],
    queryFn: ({ signal }) => {
      if (!openLibraryId) {
        return Promise.reject(new Error('openLibraryId is required'))
      }
      return fetchDiscoverWork(openLibraryId, signal)
    },
    enabled: Boolean(openLibraryId && openLibraryId.trim() !== ''),
  })
}
