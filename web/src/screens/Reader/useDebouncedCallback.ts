import { useEffect, useMemo, useRef } from 'react'

/**
 * Returns a stable debounced wrapper around `fn`. The latest `fn` is
 * always called (no stale closure); the timer is cleared on unmount.
 */
export function useDebouncedCallback<A extends unknown[]>(
  fn: (...args: A) => void,
  delayMs: number,
): (...args: A) => void {
  const fnRef = useRef(fn)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => {
    fnRef.current = fn
  }, [fn])

  useEffect(() => {
    const t = timer
    return () => clearTimeout(t.current)
  }, [])

  return useMemo(
    () =>
      (...args: A) => {
        clearTimeout(timer.current)
        timer.current = setTimeout(() => fnRef.current(...args), delayMs)
      },
    [delayMs],
  )
}
