import { Outlet } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { ContentPane } from './ContentPane'
import { Sidebar } from './Sidebar'
import { Titlebar } from './Titlebar'
import './shell.css'

/**
 * The application shell (frontend-shell-and-routing.md FR-3): Titlebar +
 * Sidebar frame a ContentPane that swaps per route. Mounted once as the
 * router's layout route — the frame never remounts on navigation, only
 * <Outlet/> changes. T6 adds the <MobileTabBar> reflow below the
 * breakpoint.
 */
export function AppShell() {
  return (
    <div className="flex h-screen flex-col bg-background text-text font-ui">
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
      <div className="flex min-h-0 flex-1">
        <Sidebar />
        <ContentPane>
          <Outlet />
        </ContentPane>
      </div>
    </div>
  )
}
