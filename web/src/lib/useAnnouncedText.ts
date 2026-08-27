import { useEffect, useRef } from 'react'

/**
 * Returns a ref to attach to a role="status"/aria-live element, which gets
 * its text content set imperatively after mount rather than rendered
 * synchronously on first paint. Many screen readers only announce a live
 * region on a DOM mutation after it's registered — content already present
 * the instant the region is inserted is not guaranteed to be announced
 * (the same reason Radix's own Toast defers its hidden announcer's text to
 * the next animation frame). A direct DOM write, not React state, avoids a
 * setState-in-effect render cascade for what is fundamentally a mutation
 * of an external system (the accessibility tree), not React-owned state.
 */
export function useAnnouncedText<T extends HTMLElement>(text: string) {
  const ref = useRef<T>(null)
  useEffect(() => {
    if (ref.current) {
      ref.current.textContent = text
    }
  }, [text])
  return ref
}
