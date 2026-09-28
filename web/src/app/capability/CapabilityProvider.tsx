import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { fetchBootstrap } from '../../data/bootstrap'
import { AUTH_CHANGE_EVENT, getAccessToken, getActiveLibraryId } from '../../data/auth'
import { CapabilityContext, type CapabilityState } from './CapabilityContext'

export function CapabilityProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState(() => getAccessToken())
  const [activeLib, setActiveLib] = useState(() => getActiveLibraryId())

  useEffect(() => {
    const handleAuth = () => {
      setToken(getAccessToken())
      setActiveLib(getActiveLibraryId())
    }
    if (typeof window !== 'undefined') {
      window.addEventListener(AUTH_CHANGE_EVENT, handleAuth)
      window.addEventListener('storage', handleAuth)
      return () => {
        window.removeEventListener(AUTH_CHANGE_EVENT, handleAuth)
        window.removeEventListener('storage', handleAuth)
      }
    }
  }, [])

  const { data, isError, refetch } = useQuery({
    queryKey: ['bootstrap', token, activeLib],
    queryFn: fetchBootstrap,
  })

  // Fail-closed: neither `error` nor `loading` ever grants a capability,
  // so host-only content still never renders optimistically. A failed fetch
  // surfaces a recoverable `error` with `retry`.
  // Memoize value and can closure to avoid re-rendering consumers.
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
