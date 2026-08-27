import { BREAKPOINTS } from '../../breakpoints'
import { useMediaQuery } from '../../lib/useMediaQuery'

export type ShellLayout = 'sidebar' | 'tabbar'

/**
 * FR-3's reflow: the persistent `<Sidebar>` at or above the reflow
 * breakpoint (a generated token — src/breakpoints.ts, from the design
 * reference), the `<MobileTabBar>` below it.
 */
export function useShellLayout(): ShellLayout {
  return useMediaQuery(`(min-width: ${BREAKPOINTS.reflow}px)`) ? 'sidebar' : 'tabbar'
}
