import { useSyncExternalStore } from 'react'

/**
 * Subscribes a component to a CSS media query, re-rendering when it flips.
 * Used by useShellLayout for the FR-3 sidebar↔tab-bar reflow. The
 * server-snapshot returns false — this app is CSR-only
 * (architecture-frontend.md FR-7) so it is never actually read, but
 * useSyncExternalStore requires it.
 */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onStoreChange) => {
      const mql = window.matchMedia(query)
      mql.addEventListener('change', onStoreChange)
      return () => mql.removeEventListener('change', onStoreChange)
    },
    () => window.matchMedia(query).matches,
    () => false,
  )
}
