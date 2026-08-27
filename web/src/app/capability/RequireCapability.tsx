import type { ReactNode } from 'react'
import type { Capability } from '../../data/bootstrap'
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

  // Unreachable this phase (the mock grants everything); the branch keeps
  // the shape ready for phase 12/13's real denied state.
  if (!state.can(capability)) {
    return <p className="text-text-2 p-3xl">This isn't available on this device.</p>
  }

  return <>{children}</>
}
