import { useContext } from 'react'
import { NavList, type NavListItem } from '../../components/NavList'
import { CapabilityContext } from '../../app/capability/CapabilityContext'
import { getCurrentUser } from '../../data/auth'

interface MoreNavItem extends NavListItem {
  requireAdmin?: boolean
  requireImport?: boolean
}

/**
 * The mobile tab bar has room for four destinations (Library, Discover,
 * Collections, More). "More" surfaces remaining application destinations,
 * strictly filtered based on the user's role and capabilities.
 */
export function MoreScreen() {
  const user = getCurrentUser()
  const isAdmin = user ? user.role === 'admin' : true
  const capability = useContext(CapabilityContext)
  const canImport = capability?.status === 'granted' ? capability.can('import') : isAdmin

  const allItems: MoreNavItem[] = [
    { to: '/settings/account', label: 'Account', description: 'Personal profile and security.' },
    { to: '/settings/devices', label: 'Devices', description: 'Paired devices and sync.' },
    { to: '/import', label: 'Import', description: 'Bring books in from a configured source.', requireImport: true },
    { to: '/sources', label: 'Sources', description: 'Where books are fetched from.', requireAdmin: true },
    { to: '/libraries', label: 'Libraries', description: 'Libraries, members, and invitations.', requireAdmin: true },
    { to: '/activity', label: 'Activity', description: 'Imports and background jobs.', requireAdmin: true },
    { to: '/settings/network', label: 'Network', description: 'Remote access and the serving address.', requireAdmin: true },
    { to: '/settings/diagnostics', label: 'Diagnostics', description: 'Server metrics and version.', requireAdmin: true },
  ]

  const items = allItems.filter((item) => {
    if (item.requireAdmin && !isAdmin) return false
    if (item.requireImport && !canImport) return false
    return true
  })

  return (
    <div className="flex flex-col gap-lg p-3xl">
      <h1 className="text-3xl font-medium tracking-1">More</h1>
      <NavList ariaLabel="More sections" items={items} />
    </div>
  )
}
