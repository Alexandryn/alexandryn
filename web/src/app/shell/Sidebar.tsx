import { Suspense, lazy, useContext } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { QueryClientContext } from '@tanstack/react-query'
import { useActivityBadge } from '../../data/activity'
import { getCurrentUser } from '../../data/auth'
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

const SidebarLive = lazy(() => import('./SidebarLive'))

function getNavIcon(to: string, isActive: boolean) {
  const iconClass = cx(
    'size-4 shrink-0 transition-colors',
    isActive ? 'text-text' : 'text-text-3 group-hover:text-text',
  )

  switch (to) {
    case '/library':
      return <BookOpenIcon className={iconClass} />
    case '/discover':
      return <CompassIcon className={iconClass} />
    case '/sources':
      return <DatabaseIcon className={iconClass} />
    case '/collections':
      return <FolderIcon className={iconClass} />
    case '/activity':
      return <ActivityIcon className={iconClass} />
    case '/import':
      return <ImportIcon className={iconClass} />
    case '/settings':
      return <SettingsIcon className={iconClass} />
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
      className="size-1.5 rounded-full bg-warm flex-none"
    />
  )
}

function SidebarStaticContent({ hasQueryClient }: { hasQueryClient: boolean }) {
  const user = getCurrentUser()
  const isAdmin = user ? user.role === 'admin' : true

  const visibleItems = NAV_ITEMS.filter((item) => {
    if (item.to === '/sources' || item.to === '/activity' || item.to === '/import') {
      return isAdmin
    }
    return true
  })

  return (
    <>
      <div className="flex items-center gap-md px-2 py-sm mb-xs">
        <div className="sidebar-book-icon" aria-hidden="true">
          <div className="sidebar-book-icon-inner" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-xl font-semibold tracking-4 text-text leading-tight truncate">
            Home Library
          </div>
        </div>
      </div>

      <ul className="flex flex-col gap-4xs flex-1">
        {visibleItems.map((item) => {
          const isSecondary =
            item.dividerBefore || item.to === '/import' || item.to === '/settings'
          return (
            <li
              key={item.to}
              className={cx(item.dividerBefore && 'my-xs border-t border-border pt-xs')}
            >
              <NavLink
                to={item.to}
                className={({ isActive }) =>
                  cx(
                    'group relative flex items-center gap-md px-md rounded-2xs font-ui transition-all cursor-pointer',
                    isSecondary ? 'sidebar-item-sm text-xl font-normal' : 'sidebar-item-md text-2xl font-semibold',
                    isActive
                      ? 'bg-surface border border-border shadow-sm text-text'
                      : 'text-text-2 hover:text-text hover:bg-surface-3/40',
                    FOCUS_RING,
                  )
                }
              >
                {({ isActive }) => (
                  <>
                    {getNavIcon(item.to, isActive)}
                    <span className={cx('flex-1 truncate', !isSecondary && 'font-semibold')}>{item.label}</span>
                    {item.count && (
                      <span aria-hidden="true" className="font-mono text-3xs text-text-3">
                        {item.count}
                      </span>
                    )}
                    {item.to === '/activity' && hasQueryClient && <ActivityBadge />}
                  </>
                )}
              </NavLink>
            </li>
          )
        })}
      </ul>

      {isAdmin && (
        <div className="rounded-md border border-border bg-surface p-md shadow-sm">
          <div className="flex items-center gap-2xs mb-2xs">
            <span className="size-1.5 rounded-full bg-success flex-none" />
            <span className="font-mono text-3xs uppercase tracking-7 text-text-2 font-medium">
              HOSTING
            </span>
          </div>
          <div className="font-mono text-xs text-text font-medium">127.0.0.1:8474</div>
          <div className="text-sm text-text-3 mt-4xs">0 devices connected</div>
          <Link
            to="/network"
            className="mt-2 block text-sm text-accent hover:underline cursor-pointer"
          >
            Open on another device →
          </Link>
        </div>
      )}
    </>
  )
}

export function Sidebar() {
  const hasQueryClient = Boolean(useContext(QueryClientContext))

  return (
    <nav
      aria-label="Primary"
      className="w-[var(--shell-sidebar-width)] shrink-0 border-r border-border bg-background p-sm flex flex-col gap-4xs overflow-hidden select-none"
    >
      {hasQueryClient ? (
        <Suspense fallback={<SidebarStaticContent hasQueryClient={hasQueryClient} />}>
          <SidebarLive />
        </Suspense>
      ) : (
        <SidebarStaticContent hasQueryClient={hasQueryClient} />
      )}
    </nav>
  )
}
