import { NavList, type NavListItem } from '../../components/NavList'
import { getCurrentUser } from '../../data/auth'

/**
 * Navigation index for /settings.
 * Split into Account Settings (all authenticated users) and
 * Instance Administration (admin only).
 */
export function SettingsIndex() {
  const user = getCurrentUser()
  const isAdmin = user?.role === 'admin'

  const accountItems: NavListItem[] = [
    {
      to: '/settings/account',
      label: 'Account Profile',
      description: 'Your username, email address, role, and two-factor authentication.',
    },
    {
      to: '/settings/devices',
      label: 'Devices',
      description: 'Paired devices and their sync activity. Revoke a device to end its access.',
    },
  ]

  const adminItems: NavListItem[] = [
    {
      to: '/settings/network',
      label: 'Network',
      description: 'How this library is reached from other devices, and the address it serves on.',
    },
    {
      to: '/libraries',
      label: 'Libraries',
      description: 'Create libraries, manage members, and send invitations.',
    },
    {
      to: '/sources',
      label: 'Sources',
      description: 'Manage local directories and OPDS catalogs for book cataloging.',
    },
    {
      to: '/activity',
      label: 'Activity',
      description: 'Monitor background worker jobs, queues, and import tasks.',
    },
    {
      to: '/settings/diagnostics',
      label: 'Diagnostics',
      description: 'Server uptime, runtime memory statistics, and version details.',
    },
  ]

  return (
    <div className="flex flex-col gap-xl p-3xl max-w-[48rem]">
      <div>
        <h1 className="text-3xl font-medium tracking-1">Settings</h1>
        <p className="text-sm text-text-2 mt-4xs">
          Personal account configuration and system preferences.
        </p>
      </div>

      <section className="flex flex-col gap-md" aria-labelledby="account-settings-heading">
        <h2 id="account-settings-heading" className="text-lg font-medium text-text">
          Account Settings
        </h2>
        <NavList ariaLabel="Account settings sections" items={accountItems} />
      </section>

      {isAdmin && (
        <section className="flex flex-col gap-md" aria-labelledby="admin-settings-heading">
          <h2 id="admin-settings-heading" className="text-lg font-medium text-text">
            Instance Administration
          </h2>
          <NavList ariaLabel="Instance administration sections" items={adminItems} />
        </section>
      )}
    </div>
  )
}
