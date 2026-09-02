import { useQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { Spinner } from '../../components/Spinner/Spinner'
import { fetchSetupStatus, getAccessToken, getRefreshToken } from '../../data/auth'

export interface RequireAuthProps {
  children: ReactNode
}

export function RequireAuth({ children }: RequireAuthProps) {
  const location = useLocation()

  const { data: status, isLoading } = useQuery({
    queryKey: ['setupStatus'],
    queryFn: fetchSetupStatus,
    staleTime: 300_000,
  })

  if (isLoading) {
    return <Spinner label="Checking system status" className="m-3xl" />
  }

  // If system has not been initialized with admin account, redirect to /setup
  if (status && !status.isSetup) {
    if (location.pathname !== '/setup') {
      return <Navigate to="/setup" replace />
    }
    return <>{children}</>
  }

  // If system is setup, require login
  const token = getAccessToken() || getRefreshToken()
  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}
