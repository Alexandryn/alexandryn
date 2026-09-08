import { NavList } from '../../components/NavList'

/**
 * The mobile tab bar has room for four destinations (Library, Discover,
 * Collections, More). "More" is where the rest of the app lives on a
 * phone — before this it was an empty <h1> and Activity, Import, Sources
 * and every settings page were unreachable on a mobile viewport (audit
 * 0016 #93).
 */
export function MoreScreen() {
  return (
    <div className="flex flex-col gap-lg p-3xl">
      <h1 className="text-3xl font-medium tracking-1">More</h1>
      <NavList
        ariaLabel="More sections"
        items={[
          { to: '/activity', label: 'Activity', description: 'Imports and background jobs.' },
          { to: '/import', label: 'Import', description: 'Bring books in from a configured source.' },
          { to: '/sources', label: 'Sources', description: 'Where books are fetched from.' },
          { to: '/libraries', label: 'Libraries', description: 'Libraries, members, and invitations.' },
          { to: '/settings/network', label: 'Network', description: 'Remote access and the serving address.' },
          { to: '/settings/devices', label: 'Devices', description: 'Paired devices and sync.' },
        ]}
      />
    </div>
  )
}
