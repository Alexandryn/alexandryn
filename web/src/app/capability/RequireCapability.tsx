import type { ReactNode } from 'react'
import { useInRouterContext, useNavigate } from 'react-router-dom'
import type { Capability } from '../../data/bootstrap'
import { getAccessToken, getCurrentUser } from '../../data/auth'
import { Button } from '../../components/Button/Button'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Spinner } from '../../components/Spinner/Spinner'
import { useCapability } from './CapabilityContext'

export interface RequireCapabilityProps {
  capability: Capability
  children: ReactNode
  /** Shown while the capability value is loading; a centred spinner by default. */
  loading?: ReactNode
}

interface CapabilityCopy {
  title: string
  cause: string
  next: string
  actionLabel: string
  actionTo: string
}

function getCapabilityCopy(capability: Capability): CapabilityCopy {
  const token = getAccessToken()
  const user = getCurrentUser()
  const isAuthenticated = Boolean(token)
  const isReader = user?.role === 'reader'

  switch (capability) {
    case 'sources':
      if (!isAuthenticated) {
        return {
          title: 'Sign in to configure sources',
          cause:
            'Sources manage the filesystem directories and OPDS catalogs indexed by Alexandryn. This requires an administrator account.',
          next: 'Sign in with an administrator account to view and modify library sources.',
          actionLabel: 'Sign in',
          actionTo: '/login',
        }
      }
      if (isReader) {
        return {
          title: 'Administrator access required for sources',
          cause:
            'Sources manage the filesystem directories and OPDS catalogs indexed by Alexandryn. Your current account has reader permissions.',
          next: 'To add or modify sources, sign in with an administrator account on this computer, or ask your library administrator.',
          actionLabel: 'Back to Library',
          actionTo: '/library',
        }
      }
      return {
        title: 'Sources can only be managed locally',
        cause:
          'Source directories and OPDS catalogs can only be configured directly on the main computer running the Alexandryn server, not from remote companion devices.',
        next: 'Open the Alexandryn desktop app or sign in with an administrator account on this computer to manage sources.',
        actionLabel: 'Back to Library',
        actionTo: '/library',
      }

    case 'import':
      if (!isAuthenticated) {
        return {
          title: 'Sign in to import books',
          cause: 'Importing book files into the library requires an authenticated account.',
          next: 'Sign in to review and import books.',
          actionLabel: 'Sign in',
          actionTo: '/login',
        }
      }
      if (isReader) {
        return {
          title: 'Reader uploads are disabled',
          cause: 'This library is configured to disallow reader accounts from uploading book files.',
          next: 'Ask a library administrator to enable reader uploads in Library Settings, or sign in with an administrator account.',
          actionLabel: 'Back to Library',
          actionTo: '/library',
        }
      }
      return {
        title: 'Book import is restricted to the host machine',
        cause:
          'Importing files directly is only permitted on the host computer running the Alexandryn server.',
        next: 'Open Alexandryn on the host computer to import books.',
        actionLabel: 'Back to Library',
        actionTo: '/library',
      }

    case 'network':
      if (!isAuthenticated) {
        return {
          title: 'Sign in to configure network settings',
          cause:
            'Network binding, TLS certificates, and device pairing require administrator authentication.',
          next: 'Sign in with an administrator account to manage network exposure.',
          actionLabel: 'Sign in',
          actionTo: '/login',
        }
      }
      if (isReader) {
        return {
          title: 'Administrator access required for network settings',
          cause:
            'Network settings are restricted to administrators. Reader accounts cannot change server binding or device pairings.',
          next: 'Sign in with an administrator account on the host machine to manage network settings.',
          actionLabel: 'Back to Library',
          actionTo: '/library',
        }
      }
      return {
        title: 'Network settings are restricted to the host machine',
        cause:
          'Network binding, TLS, and pairing configuration must be managed directly on the host computer running the Alexandryn server.',
        next: 'Open Alexandryn on the host machine to change network exposure.',
        actionLabel: 'Back to Library',
        actionTo: '/library',
      }

    case 'settings':
      if (!isAuthenticated) {
        return {
          title: 'Sign in to access settings',
          cause:
            'Managing library settings, paired devices, and server preferences requires administrator authentication.',
          next: 'Sign in with an administrator account to view and edit settings.',
          actionLabel: 'Sign in',
          actionTo: '/login',
        }
      }
      if (isReader) {
        return {
          title: 'Administrator access required for settings',
          cause: 'Device and library settings can only be viewed and changed by an administrator.',
          next: 'Sign in with an administrator account on the host machine.',
          actionLabel: 'Back to Library',
          actionTo: '/library',
        }
      }
      return {
        title: 'Settings are restricted to the host machine',
        cause:
          'Configuration for this section must be accessed directly on the host computer running Alexandryn.',
        next: 'Open Alexandryn on the host machine.',
        actionLabel: 'Back to Library',
        actionTo: '/library',
      }

    case 'system':
    default:
      if (!isAuthenticated) {
        return {
          title: 'Sign in to access system diagnostics',
          cause:
            'Server logs, process metrics, and queue diagnostics require administrator authentication.',
          next: 'Sign in with an administrator account.',
          actionLabel: 'Sign in',
          actionTo: '/login',
        }
      }
      if (isReader) {
        return {
          title: 'Administrator access required for system diagnostics',
          cause:
            'System diagnostics are restricted to administrators. Reader accounts cannot view server metrics or worker queues.',
          next: 'Sign in with an administrator account on the host computer.',
          actionLabel: 'Back to Library',
          actionTo: '/library',
        }
      }
      return {
        title: 'System diagnostics are restricted to the host machine',
        cause:
          'Server diagnostics can only be accessed directly on the host computer running the Alexandryn server.',
        next: 'Open Alexandryn directly on the host machine.',
        actionLabel: 'Back to Library',
        actionTo: '/library',
      }
  }
}

function RouterActionButton({ to, label }: { to: string; label: string }) {
  const navigate = useNavigate()
  return (
    <Button variant="secondary" size="sm" onClick={() => navigate(to)}>
      {label}
    </Button>
  )
}

function ActionButton({ to, label }: { to: string; label: string }) {
  const inRouter = useInRouterContext()
  if (inRouter) {
    return <RouterActionButton to={to} label={label} />
  }
  return (
    <Button
      variant="secondary"
      size="sm"
      onClick={() => {
        if (typeof window !== 'undefined') {
          window.location.assign(to)
        }
      }}
    >
      {label}
    </Button>
  )
}

/**
 * Wraps a host-only route's content. While the capability value is
 * loading it renders the loading state, never the children — host-only
 * content must not render optimistically then disappear.
 */
export function RequireCapability({ capability, children, loading }: RequireCapabilityProps) {
  const state = useCapability()

  if (state.status === 'loading') {
    return <>{loading ?? <Spinner label="Checking what this device can do" className="m-3xl" />}</>
  }

  if (state.status === 'error') {
    return (
      <ErrorState
        title="Couldn't load what this device can do"
        description="The app needs to reach the server to know which screens to show. Check your connection and try again."
        onRetry={state.retry}
      />
    )
  }

  // Denied capability branch.
  if (!state.can(capability)) {
    const copy = getCapabilityCopy(capability)
    return (
      <div className="mx-auto flex max-w-[36rem] flex-col items-center gap-xs p-3xl text-center">
        <h1 className="text-2xl font-serif font-bold text-text mb-xs">{copy.title}</h1>
        <p className="text-sm text-text-2 mb-2xs">{copy.cause}</p>
        <p className="text-xs text-text-3 mb-md">{copy.next}</p>
        <ActionButton to={copy.actionTo} label={copy.actionLabel} />
      </div>
    )
  }

  return <>{children}</>
}
