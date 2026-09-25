import { useContext } from 'react'
import { NavLink } from 'react-router-dom'
import { QueryClientContext } from '@tanstack/react-query'
import { useActivityBadge } from '../../data/activity'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { NAV_ITEMS } from './navItems'
import {
  BookOpenIcon,
  CompassIcon,
  DatabaseIcon,
  FolderIcon,
  ActivityIcon,
  ImportIcon,
  SettingsIcon,
} from '../../components/Icon'

const LINK_BASE = cx(
  'group flex items-center gap-3 h-10 px-3 rounded-md text-sm font-medium font-ui transition-colors',
  'text-text-2 hover:text-text hover:bg-surface-3',
  FOCUS_RING,
)

function getNavIcon(to: string) {
  switch (to) {
    case '/library':
      return <BookOpenIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    case '/discover':
      return <CompassIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    case '/sources':
      return <DatabaseIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    case '/collections':
      return <FolderIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    case '/activity':
      return <ActivityIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    case '/import':
      return <ImportIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    case '/settings':
      return <SettingsIcon className="size-5 shrink-0 text-text-3 group-hover:text-text transition-colors" />
    default:
      return null
  }
}

function ActivityBadge() {
  const { hasActiveOrFailed } = useActivityBadge()
  if (!hasActiveOrFailed) return null
  return (
    <span
      role="status"
      aria-label="Activity — action required"
      className="w-2 h-2 rounded-full bg-[var(--warm)] flex-none"
    />
  )
}

/**
 * The persistent desktop navigation rail. A real <nav> landmark, not a
 * <div> with role. Rendered only at/above the reflow breakpoint; below it
 * the <MobileTabBar> replaces it entirely.
 */
export function Sidebar() {
  const hasQueryClient = Boolean(useContext(QueryClientContext))

  return (
    <nav
      aria-label="Primary"
      className="w-[var(--shell-sidebar-width)] shrink-0 border-r border-border bg-surface px-md py-lg"
    >
      <ul className="flex flex-col gap-1">
        {NAV_ITEMS.map((item) => (
          <li
            key={item.to}
            className={cx(item.dividerBefore && 'mt-sm border-t border-border pt-sm')}
          >
            <NavLink
              to={item.to}
              className={({ isActive }) =>
                cx(
                  LINK_BASE,
                  isActive && 'bg-surface-3 text-text font-semibold shadow-2xs border-l-2 border-accent',
                )
              }
            >
              {getNavIcon(item.to)}
              <span className="flex-1 text-sm font-medium">{item.label}</span>
              {item.to === '/activity' && hasQueryClient && <ActivityBadge />}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
