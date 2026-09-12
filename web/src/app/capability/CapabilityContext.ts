import { createContext, useContext } from 'react'
import type { Capability } from '../../data/bootstrap'

/**
 * Capability-gating shape. `loading` until a value arrives (never
 * instant, even against the mock); `granted` once it has.
 */
export type CapabilityState =
  | { status: 'loading' }
  | { status: 'granted'; can: (capability: Capability) => boolean }
  // The bootstrap fetch failed. This is NOT `granted` — the fail-closed
  // property holds (host-only content still never renders) — but the UI
  // shows a recoverable error with `retry` instead of an eternal spinner.
  | { status: 'error'; retry: () => void }

export const CapabilityContext = createContext<CapabilityState | null>(null)

export function useCapability(): CapabilityState {
  const ctx = useContext(CapabilityContext)
  if (ctx === null) {
    throw new Error('useCapability must be used within <CapabilityProvider>')
  }
  return ctx
}
