import type { ReactNode } from 'react'

/**
 * The routed region. A real <main> landmark; `id="main"` and
 * `tabIndex={-1}` make it the target for the skip link and for
 * post-navigation focus moves. The shell frame around it never remounts —
 * only this element's children change.
 */
export function ContentPane({ children }: { children: ReactNode }) {
  return (
    <main
      id="main"
      tabIndex={-1}
      aria-label="Main content"
      className="flex-1 overflow-y-auto bg-background outline-none"
    >
      {children}
    </main>
  )
}
