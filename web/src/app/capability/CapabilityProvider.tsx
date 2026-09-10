import { useMemo, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchBootstrap } from '../../data/bootstrap'
import { CapabilityContext, type CapabilityState } from './CapabilityContext'

export function CapabilityProvider({ children }: { children: ReactNode }) {
  const { data, isError, refetch } = useQuery({ queryKey: ['bootstrap'], queryFn: fetchBootstrap })

  // Fail-closed: neither `error` nor `loading` ever grants a capability,
  // so host-only content still never renders optimistically. The only
  // change from a stuck spinner is that a failed fetch is now a
  // recoverable `error` with `retry` (audit 0016 #92).
  // Memoize value and can closure to avoid re-rendering consumers (audit 0016 #216).
  const value = useMemo<CapabilityState>(() => {
    if (data !== undefined) {
      return { status: 'granted', can: (capability) => data.capabilities[capability] === true }
    }
    if (isError) {
      return { status: 'error', retry: () => void refetch() }
    }
    return { status: 'loading' }
  }, [data, isError, refetch])

  return <CapabilityContext.Provider value={value}>{children}</CapabilityContext.Provider>
}
