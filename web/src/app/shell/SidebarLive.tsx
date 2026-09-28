import { useContext } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useActivityBadge } from '../../data/activity'
import { CapabilityContext } from '../capability/CapabilityContext'
import { getCurrentUser, getActiveLibraryId } from '../../data/auth'
import { useCollections } from '../../data/collections'
import { useDevices } from '../../data/devices'
import { fetchLibraries } from '../../data/libraries'
import { useLibrary } from '../../data/library'
import { useNetworkStatus } from '../../data/network'
import { useSources } from '../../data/sources'
import { LibrarySwitcher } from '../../screens/Libraries/LibrarySwitcher'
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

function SidebarLiveHeader() {
  const user = getCurrentUser()
  const isAdmin = user ? user.role === 'admin' : true

  const { data: librariesData } = useQuery({
    queryKey: ['libraries'],
    queryFn: fetchLibraries,
    staleTime: 60_000,
  })
  const { data: sources = [], isPending: isSourcesPending } = useSources({ enabled: isAdmin })
  const { data: libraryData, isPending: isLibraryPending } = useLibrary({ filter: 'all' })

  const libraries = librariesData?.libraries ?? []
  const activeLibId = getActiveLibraryId() || (libraries[0]?.id ?? null)
  const activeLibrary = libraries.find((l) => l.id === activeLibId) ?? libraries[0]
  const libraryName = activeLibrary?.name ?? 'Home Library'

  const booksCount =
    libraryData?.pages.reduce((acc, p) => acc + p.works.length, 0) ?? 0
  const sourcesCount = sources.length

  let statsText: string
  if (isAdmin) {
    statsText = isSourcesPending && isLibraryPending
      ? '...'
      : `${booksCount.toLocaleString()} ${booksCount === 1 ? 'BOOK' : 'BOOKS'} · ${sourcesCount.toLocaleString()} ${sourcesCount === 1 ? 'SOURCE' : 'SOURCES'}`
  } else {
    statsText = isLibraryPending
      ? '...'
      : `${booksCount.toLocaleString()} ${booksCount === 1 ? 'BOOK' : 'BOOKS'}`
  }

  return (
    <div className="flex flex-col gap-xs px-2 py-sm mb-xs">
      <div className="flex items-center gap-md">
        <div className="sidebar-book-icon" aria-hidden="true">
          <div className="sidebar-book-icon-inner" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-xl font-semibold tracking-4 text-text leading-tight truncate">
            {libraryName}
          </div>
          <div className="font-mono text-3xs text-text-3 tracking-5 truncate">
            {statsText}
          </div>
        </div>
      </div>
      <LibrarySwitcher />
    </div>
  )
}

function SidebarLiveNavList() {
  const user = getCurrentUser()
  const isAdmin = user ? user.role === 'admin' : true
  const capability = useContext(CapabilityContext)
  const canImport = capability?.status === 'granted' ? capability.can('import') : isAdmin

  const { data: sources = [] } = useSources({ enabled: isAdmin })
  const { data: collections = [] } = useCollections()
  const { data: libraryData } = useLibrary({ filter: 'all' })

  const booksCount =
    libraryData?.pages.reduce((acc, p) => acc + p.works.length, 0) ?? 0

  const getDynamicBadge = (to: string): number | string | undefined => {
    if (to === '/library' && booksCount > 0) return booksCount
    if (to === '/sources' && sources.length > 0) return sources.length
    if (to === '/collections' && collections.length > 0) return collections.length
    return undefined
  }

  const visibleItems = NAV_ITEMS.filter((item) => {
    if (item.to === '/sources' || item.to === '/activity') {
      return isAdmin
    }
    if (item.to === '/import') {
      return canImport
    }
    return true
  })

  return (
    <ul className="flex flex-col gap-4xs flex-1">
      {visibleItems.map((item) => {
        const isSecondary =
          item.dividerBefore || item.to === '/import' || item.to === '/settings'
        const badge = getDynamicBadge(item.to) ?? item.count
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
                  {badge && (
                    <span aria-hidden="true" className="font-mono text-3xs text-text-3">
                      {badge}
                    </span>
                  )}
                  {item.to === '/activity' && <ActivityBadge />}
                </>
              )}
            </NavLink>
          </li>
        )
      })}
    </ul>
  )
}

function SidebarLiveHostingCard() {
  const { data: networkStatus } = useNetworkStatus()
  const { data: devicesData } = useDevices()

  const address =
    networkStatus?.address ||
    (typeof window !== 'undefined' && window.location?.host
      ? window.location.host
      : '127.0.0.1:8474')

  const devicesCount = devicesData?.devices?.length ?? 0
  const devicesText = `${devicesCount} ${devicesCount === 1 ? 'device' : 'devices'} connected`

  return (
    <div className="rounded-md border border-border bg-surface p-md shadow-sm">
      <div className="flex items-center gap-2xs mb-2xs">
        <span className="size-1.5 rounded-full bg-success flex-none" />
        <span className="font-mono text-3xs uppercase tracking-7 text-text-2 font-medium">
          HOSTING
        </span>
      </div>
      <div className="font-mono text-xs text-text font-medium truncate">{address}</div>
      <div className="text-sm text-text-3 mt-4xs">{devicesText}</div>
      <Link
        to="/network"
        className="mt-2 block text-sm text-accent hover:underline cursor-pointer"
      >
        Open on another device →
      </Link>
    </div>
  )
}

export default function SidebarLive() {
  const user = getCurrentUser()
  const isAdmin = user ? user.role === 'admin' : true

  return (
    <>
      <SidebarLiveHeader />
      <SidebarLiveNavList />
      {isAdmin && <SidebarLiveHostingCard />}
    </>
  )
}
