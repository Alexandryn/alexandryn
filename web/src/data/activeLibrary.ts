import type { QueryClient } from '@tanstack/react-query'
import { setActiveLibraryId } from './auth'

// Query-key roots that are NOT scoped to the active library. Everything
// else is assumed library-scoped (the X-Library-Id header the http client
// attaches makes almost every response tenant-specific) and is dropped
// on a switch.
const LIBRARY_INDEPENDENT_ROOTS = new Set([
  'libraries', // the set of libraries the user can reach
  'bootstrap', // host capabilities
  'devices', // the user's paired devices
  'discover', // Open Library — no tenant
  'network', // host network status
])

/**
 * Switch the active library and evict every cached query that was scoped
 * to the previous one, so no screen keeps showing another library's data
 * after the switch (audit 0016 #138). The earlier call site invalidated
 * only three hardcoded keys and missed work detail, sources, the activity
 * feed, the reader and library members.
 */
export function switchActiveLibrary(queryClient: QueryClient, id: string): void {
  setActiveLibraryId(id)
  void queryClient.invalidateQueries({
    predicate: (query) => !LIBRARY_INDEPENDENT_ROOTS.has(String(query.queryKey[0])),
  })
}
