import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from '../data/http'
import { clearSession, refreshSession } from '../data/auth'

/** A 4xx is the caller's fault, not a transient failure — never retry it. */
function isClientError(error: unknown): boolean {
  return error instanceof ApiError && error.status >= 400 && error.status < 500
}

// One in-flight refresh shared by every 401 that arrives while it runs,
// so a burst of failed queries triggers a single POST /auth/refresh.
let refreshInFlight: Promise<boolean> | null = null

// Consecutive refresh-then-refetch cycles with no successful response in
// between. A refresh that returns 200 without a usable token, or a fresh
// token the server still rejects (clock skew, signing-key rotation),
// would otherwise re-enter handleAuthFailure forever — hammering
// /auth/refresh and refetching every query. After this many, stop and
// send the user to login. Reset to 0 by the first successful response.
const MAX_REFRESH_CYCLES = 2
let refreshCycles = 0

function refreshOnce(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = refreshSession()
      // A refresh that resolves without minting an access token is a
      // failed refresh, not a successful one.
      .then((res) => Boolean(res?.accessToken))
      .catch(() => false)
      .finally(() => {
        refreshInFlight = null
      })
  }
  return refreshInFlight
}

let clientRef: QueryClient | null = null

/**
 * Handles a 401 from any query or mutation. Before this,
 * an expired access token left every screen showing ErrorState with a
 * dead "Try again" and no path back to login. Now: try one refresh; on
 * success, refetch; on failure, clear the session and send the user to
 * /login with the current location preserved as ?next.
 */
async function handleAuthFailure(error: unknown): Promise<void> {
  if (!(error instanceof ApiError) || error.status !== 401) return
  if (typeof window === 'undefined') return
  if (window.location.pathname === '/login' || window.location.pathname === '/setup') return

  if (refreshCycles < MAX_REFRESH_CYCLES && (await refreshOnce())) {
    refreshCycles++
    void clientRef?.invalidateQueries()
    return
  }

  refreshCycles = 0
  clearSession()
  const here = window.location.pathname + window.location.search
  window.location.assign(`/login?next=${encodeURIComponent(here)}`)
}

/**
 * Builds a QueryClient with this app's defaults made explicit rather than
 * inherited silently: a transient failure retries with capped backoff
 * instead of blanking cached data, a 4xx never retries, a short staleTime
 * keeps stale-while-revalidate the visible default, and a 401 anywhere is
 * handled globally.
 */
export function makeQueryClient(): QueryClient {
  const client = new QueryClient({
    queryCache: new QueryCache({
      onError: (error) => void handleAuthFailure(error),
      // A response that lands proves the current token is good — clear
      // the refresh circuit breaker.
      onSuccess: () => {
        refreshCycles = 0
      },
    }),
    mutationCache: new MutationCache({
      onError: (error) => void handleAuthFailure(error),
      onSuccess: () => {
        refreshCycles = 0
      },
    }),
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
