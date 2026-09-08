import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from '../data/http'
import { clearSession, refreshSession } from '../data/auth'

/** A 4xx is the caller's fault, not a transient failure — never retry it. */
function isClientError(error: unknown): boolean {
  return error instanceof ApiError && error.status >= 400 && error.status < 500
}

// One in-flight refresh shared by every 401 that arrives while it runs,
// so a burst of failed queries triggers a single POST /auth/refresh
// (audit 0016 #91).
let refreshInFlight: Promise<boolean> | null = null

function refreshOnce(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = refreshSession()
      .then(() => true)
      .catch(() => false)
      .finally(() => {
        refreshInFlight = null
      })
  }
  return refreshInFlight
}

let clientRef: QueryClient | null = null

/**
 * Handles a 401 from any query or mutation (audit 0016 #91). Before this,
 * an expired access token left every screen showing ErrorState with a
 * dead "Try again" and no path back to login. Now: try one refresh; on
 * success, refetch; on failure, clear the session and send the user to
 * /login with the current location preserved as ?next.
 */
async function handleAuthFailure(error: unknown): Promise<void> {
  if (!(error instanceof ApiError) || error.status !== 401) return
  if (typeof window === 'undefined') return
  if (window.location.pathname === '/login' || window.location.pathname === '/setup') return

  if (await refreshOnce()) {
    void clientRef?.invalidateQueries()
    return
  }

  clearSession()
  const here = window.location.pathname + window.location.search
  window.location.assign(`/login?next=${encodeURIComponent(here)}`)
}

/**
 * Builds a QueryClient with this app's defaults made explicit rather than
 * inherited silently (frontend-shell-and-routing.md FR-2): a transient
 * failure retries with capped backoff instead of blanking cached data, a
 * 4xx never retries, a short staleTime keeps stale-while-revalidate the
 * visible default, and a 401 anywhere is handled globally.
 */
export function makeQueryClient(): QueryClient {
  const client = new QueryClient({
    queryCache: new QueryCache({ onError: (error) => void handleAuthFailure(error) }),
    mutationCache: new MutationCache({ onError: (error) => void handleAuthFailure(error) }),
    defaultOptions: {
      queries: {
        retry: (failureCount, error) => !isClientError(error) && failureCount < 2,
        retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 5000),
        staleTime: 30_000,
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: false,
      },
    },
  })
  clientRef = client
  return client
}
