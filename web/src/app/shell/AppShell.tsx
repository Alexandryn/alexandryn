import { Suspense } from 'react'
import { Outlet } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { Spinner } from '../../components/Spinner/Spinner'
import { FOCUS_RING } from '../../lib/focusRing'
import { ContentPane } from './ContentPane'
import { MobileTabBar } from './MobileTabBar'
import { Sidebar } from './Sidebar'
import { Titlebar } from './Titlebar'
import { useContentFocusOnRouteChange } from './useContentFocusOnRouteChange'
import { useShellLayout } from './useShellLayout'
import './shell.css'

/**
 * The application shell (frontend-shell-and-routing.md FR-3): Titlebar +
 * Sidebar frame a ContentPane that swaps per route. Mounted once as the
 * router's layout route — the frame never remounts on navigation, only
 * <Outlet/> changes. Below the reflow breakpoint the <Sidebar> is
 * replaced by a bottom <MobileTabBar> (FR-3); the <ContentPane> keeps its
 * position across that swap, so a viewport reflow never remounts the
 * routed content, only the navigation element changes.
 */
export function AppShell() {
  const layout = useShellLayout()
  useContentFocusOnRouteChange()

  return (
    <div className="flex h-dvh flex-col bg-background text-text font-ui">
      <a
        href="#main"
        className={cx(
          'sr-only focus:not-sr-only focus:absolute focus:z-10 focus:m-xs',
          'focus:rounded-2xs focus:bg-surface focus:px-md focus:py-xs focus:text-text',
          FOCUS_RING,
        )}
      >
        Skip to content
      </a>
      <Titlebar />
      <div className={cx('flex min-h-0 flex-1', layout === 'tabbar' && 'flex-col')}>
        {layout === 'sidebar' && <Sidebar />}
        <ContentPane>
          <Suspense fallback={<Spinner label="Loading" className="m-3xl" />}>
            <Outlet />
          </Suspense>
        </ContentPane>
        {layout === 'tabbar' && <MobileTabBar />}
      </div>
    </div>
  )
}
