import { BREAKPOINTS } from '../../breakpoints'
import { useMediaQuery } from '../../lib/useMediaQuery'

export type ShellLayout = 'sidebar' | 'tabbar'

/**
 * Responsive reflow: the persistent `<Sidebar>` at or above the reflow
 * breakpoint, the `<MobileTabBar>` below it.
 */
export function useShellLayout(): ShellLayout {
  return useMediaQuery(`(min-width: ${BREAKPOINTS.reflow}px)`) ? 'sidebar' : 'tabbar'
}
