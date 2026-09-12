import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { deleteRequest, getJson, patchJson, postJson } from './http'
import type { WorkSummary } from './library'

export interface CollectionSummary {
  id: string
  name: string
  workCount: number
}

export interface CollectionDetail {
  id: string
  name: string
  works: WorkSummary[]
}

export interface CollectionCreateRequest {
  name: string
}

export interface CollectionRenameRequest {
  name: string
}

export interface CollectionMembershipAddRequest {
  workId: string
}

export async function fetchCollections(): Promise<CollectionSummary[]> {
  const data = await getJson<{ collections: CollectionSummary[] }>('/api/v1/collections')
  return data.collections
}

export function fetchCollection(id: string): Promise<CollectionDetail> {
  return getJson<CollectionDetail>(`/api/v1/collections/${encodeURIComponent(id)}`)
}

export function createCollection(name: string): Promise<CollectionDetail> {
  return postJson<CollectionDetail>('/api/v1/collections', { name })
}

export function renameCollection(id: string, name: string): Promise<CollectionDetail> {
  return patchJson<CollectionDetail>(`/api/v1/collections/${encodeURIComponent(id)}`, { name })
}

export function deleteCollection(id: string): Promise<void> {
  return deleteRequest(`/api/v1/collections/${encodeURIComponent(id)}`)
}

export function addWorkToCollection(
  collectionId: string,
  workId: string,
): Promise<CollectionDetail> {
  return postJson<CollectionDetail>(
    `/api/v1/collections/${encodeURIComponent(collectionId)}/works`,
    { workId },
  )
}

export function removeWorkFromCollection(
  collectionId: string,
  workId: string,
): Promise<void> {
  return deleteRequest(
    `/api/v1/collections/${encodeURIComponent(collectionId)}/works/${encodeURIComponent(workId)}`,
  )
}

/**
 * Collections list hook using TanStack Query useQuery.
 * Cache key: ['collections'].
 */
export function useCollections() {
  return useQuery({
    queryKey: ['collections'],
    queryFn: fetchCollections,
  })
}

/**
 * Single collection detail hook using TanStack Query useQuery.
 * Cache key: ['collection', id].
 */
export function useCollection(id: string | undefined) {
  return useQuery({
    queryKey: ['collection', id],
    queryFn: () => (id ? fetchCollection(id) : Promise.reject(new Error('id is required'))),
    enabled: Boolean(id && id.trim() !== ''),
  })
}

/**
 * Mutation for creating a new collection.
 * Invalidates ['collections'] on success.
 */
export function useCreateCollection() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => createCollection(name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
    },
  })
}

/**
 * Mutation for renaming a collection.
 * Invalidates ['collections'] and ['collection', id] on success.
 */
export function useRenameCollection() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => renameCollection(id, name),
    onSuccess: (data) => {
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['collection', data.id] })
      void queryClient.invalidateQueries({ queryKey: ['library'] })
    },
  })
}

/**
 * Mutation for deleting a collection.
 * Invalidates ['collections'] and removes ['collection', id] on success.
 */
export function useDeleteCollection() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => deleteCollection(id),
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['library'] })
      queryClient.removeQueries({ queryKey: ['collection', id] })
    },
  })
}

/**
 * Mutation for adding a work to a collection.
 * Invalidates ['collections'], ['collection', collectionId], ['work', workId], and ['library'].
 */
export function useAddWorkToCollection() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ collectionId, workId }: { collectionId: string; workId: string }) =>
      addWorkToCollection(collectionId, workId),
    onSuccess: (_data, { collectionId, workId }) => {
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['collection', collectionId] })
      void queryClient.invalidateQueries({ queryKey: ['work', workId] })
      void queryClient.invalidateQueries({ queryKey: ['library'] })
    },
  })
}

/**
 * Mutation for removing a work from a collection.
 * Invalidates ['collections'], ['collection', collectionId], ['work', workId], and ['library'].
 */
export function useRemoveWorkFromCollection() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ collectionId, workId }: { collectionId: string; workId: string }) =>
      removeWorkFromCollection(collectionId, workId),
    onSuccess: (_data, { collectionId, workId }) => {
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['collection', collectionId] })
      void queryClient.invalidateQueries({ queryKey: ['work', workId] })
      void queryClient.invalidateQueries({ queryKey: ['library'] })
    },
  })
}

