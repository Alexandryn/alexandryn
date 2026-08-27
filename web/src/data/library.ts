import { useQuery } from '@tanstack/react-query'
import { getJson } from './http'

// Minimal shape for phase 04's shell demo — phase 06 replaces this with
// the real library resource once api/openapi.yaml defines it.
export interface LibraryItem {
  id: string
  title: string
  author: string
}

function fetchLibraryItems(): Promise<LibraryItem[]> {
  return getJson<LibraryItem[]>('/api/v1/library')
}

export function useLibraryItems() {
  return useQuery({ queryKey: ['library'], queryFn: fetchLibraryItems })
}
