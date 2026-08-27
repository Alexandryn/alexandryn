import type { ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchBootstrap } from '../../data/bootstrap'
import { CapabilityContext, type CapabilityState } from './CapabilityContext'

export function CapabilityProvider({ children }: { children: ReactNode }) {
  const { data } = useQuery({ queryKey: ['bootstrap'], queryFn: fetchBootstrap })

  // `data === undefined` covers both the initial load and a failed fetch
  // — the spec designs no fallback timeout, so a stuck value stays
  // `loading` rather than degrading to an optimistic `granted`.
  const value: CapabilityState =
    data === undefined
      ? { status: 'loading' }
      : { status: 'granted', can: (capability) => data.capabilities[capability] === true }

  return <CapabilityContext.Provider value={value}>{children}</CapabilityContext.Provider>
}
