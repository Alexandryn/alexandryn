import { useEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'

/**
 * On a client-side navigation (a real pathname change), moves keyboard
 * focus to the content region so a screen-reader or keyboard user is
 * placed in the new screen rather than left where the old page's link
 * was. The initial load is skipped on purpose — moving focus there would
 * jump past the skip link before the user could use it.
 *
 * Tracks the previous pathname rather than a "first render" flag so
 * StrictMode's double-invoked effect can't mistake the second mount at
 * the same URL for a navigation.
 */
export function useContentFocusOnRouteChange() {
  const { pathname } = useLocation()
  const previous = useRef<string | null>(null)

  useEffect(() => {
    if (previous.current !== null && previous.current !== pathname) {
      document.getElementById('main')?.focus()
    }
    previous.current = pathname
  }, [pathname])
}
