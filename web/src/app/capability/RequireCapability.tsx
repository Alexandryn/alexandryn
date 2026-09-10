import type { ReactNode } from 'react'
import type { Capability } from '../../data/bootstrap'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Spinner } from '../../components/Spinner/Spinner'
import { useCapability } from './CapabilityContext'

export interface RequireCapabilityProps {
  capability: Capability
  children: ReactNode
  /** Shown while the capability value is loading; a centred spinner by default. */
  loading?: ReactNode
}

/**
 * Wraps a host-only route's content. While the capability value is
 * loading it renders the loading state, never the children — the
 * illegal-transition rule from architecture-frontend.md (host-only
 * content must not render optimistically then disappear).
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

  // Unreachable this phase (the mock grants everything); the branch keeps
  // the shape ready for phase 12/13's real denied state.
  if (!state.can(capability)) {
    return (
      <div className="p-3xl">
        <h1 className="text-xl font-medium text-text mb-sm">Not Available</h1>
        <p className="text-text-2">This isn't available on this device.</p>
      </div>
    )
  }

  return <>{children}</>
}
