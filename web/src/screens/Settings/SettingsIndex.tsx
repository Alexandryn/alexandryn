import { NavList } from '../../components/NavList'

/**
 * The /settings index (audit 0016 #93) — before this it was a lone <h1>
 * and the real settings pages were reachable only by typing their URLs.
 */
export function SettingsIndex() {
  return (
    <div className="flex flex-col gap-lg p-3xl">
      <h1 className="text-3xl font-medium tracking-1">Settings</h1>
      <NavList
        ariaLabel="Settings sections"
        items={[
          {
            to: '/settings/network',
            label: 'Network',
            description: 'How this library is reached from other devices, and the address it serves on.',
          },
          {
            to: '/settings/devices',
            label: 'Devices',
            description: 'Paired devices and their sync activity. Revoke a device to end its access.',
          },
          {
            to: '/libraries',
            label: 'Libraries',
            description: 'Create libraries, manage members, and send invitations.',
          },
        ]}
      />
    </div>
  )
}
