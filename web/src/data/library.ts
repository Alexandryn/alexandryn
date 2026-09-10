import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { getJson } from './http'

export type LibraryFilter = 'all' | 'owned' | 'wanted'
export type LibrarySort = 'added_at' | 'title'

export interface CollectionRef {
  id: string
  name: string
  addedAt?: string | null
}

export interface WorkSummary {
  id: string
  title: string
  subtitle: string
  authors: string[]
  isOwned: boolean
  collections: CollectionRef[]
  addedAt?: string | null
}

export interface LibraryPage {
  works: WorkSummary[]
  nextCursor: string | null
}

export interface OwnedEdition {
  id: string
  language: string
  isbn?: string | null
  publisher: string
  publicationYear?: number | null
  addedAt?: string | null
  formats: string[]
}

export interface WorkDetail {
  id: string
  title: string
  subtitle: string
  authors: string[]
  subjects?: string[]
  originalLanguage?: string | null
  ownedEditions: OwnedEdition[]
  collections: CollectionRef[]
}

export interface LibraryQueryParams {
  q?: string
  filter?: LibraryFilter
  sort?: LibrarySort
  limit?: number
  cursor?: string
}

export function fetchLibraryPage(
  params: LibraryQueryParams = {},
  signal?: AbortSignal,
): Promise<LibraryPage> {
  const search = new URLSearchParams()
  if (params.q && params.q.trim() !== '') {
    search.set('q', params.q.trim())
  }
  if (params.filter && params.filter !== 'all') {
    search.set('filter', params.filter)
  }
  if (params.sort && params.sort !== 'added_at') {
    search.set('sort', params.sort)
  }
  if (params.limit && params.limit > 0) {
    search.set('limit', String(params.limit))
  }
  if (params.cursor) {
    search.set('cursor', params.cursor)
  }

  const qs = search.toString()
  const path = `/api/v1/library${qs ? `?${qs}` : ''}`
  return getJson<LibraryPage>(path, {}, { signal })
}

/**
 * Cursor-paginated library hook using TanStack Query's useInfiniteQuery (FR-1, FR-3).
 * Query key follows ['library', { q, filter, sort }] cache convention.
 */
export function useLibrary(params: {
  q?: string
  filter?: LibraryFilter
  sort?: LibrarySort
  limit?: number
} = {}) {
  const q = params.q?.trim() || undefined
  const filter = params.filter ?? 'all'
  const sort = params.sort ?? 'added_at'
  const limit = params.limit

  return useInfiniteQuery({
    queryKey: ['library', { q, filter, sort, limit }],
    // A superseded search (the user kept typing) or an unmount aborts the
    // in-flight request rather than downloading a response nobody will see
    // (audit 0016 #166).
    queryFn: ({ pageParam, signal }) =>
      fetchLibraryPage(
        {
          q,
          filter,
          sort,
          limit,
          cursor: pageParam,
        },
        signal,
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.nextCursor ?? undefined,
  })
}

export function fetchWorkDetail(id: string, signal?: AbortSignal): Promise<WorkDetail> {
  return getJson<WorkDetail>(`/api/v1/works/${encodeURIComponent(id)}`, {}, { signal })
}

/**
 * Work detail hook using TanStack Query's useQuery (FR-5).
 * Query key follows ['work', id] cache convention.
 */
export function useWork(id: string | undefined) {
  return useQuery({
    queryKey: ['work', id],
    queryFn: ({ signal }) =>
      id ? fetchWorkDetail(id, signal) : Promise.reject(new Error('id is required')),
    enabled: Boolean(id && id.trim() !== ''),
  })
}

