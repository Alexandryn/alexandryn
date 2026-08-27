import { QueryClient } from '@tanstack/react-query'

/**
 * Builds a QueryClient with this app's defaults made explicit rather than
 * inherited silently (frontend-shell-and-routing.md FR-2): a transient
 * failure retries with capped backoff instead of blanking cached data,
 * and a short staleTime keeps stale-while-revalidate the visible default.
 * A factory, not a shared singleton, so each entry point (and each test)
 * owns an isolated cache.
 */
export function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: 2,
        retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 5000),
        staleTime: 30_000,
        refetchOnWindowFocus: false,
      },
    },
  })
}
