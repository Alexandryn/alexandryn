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

  const {
    data: status,
    isLoading,
    isError,
    refetch,
  } = useQuery({
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

  // If status query failed (e.g. backend temporarily restarting or unreachable during boot),
  // don't drop to login; show retry state.
  if (isError) {
    return (
      <main className="min-h-screen flex items-center justify-center bg-background p-md">
        <div className="w-full max-w-[28rem] bg-surface p-xl rounded-lg border border-border shadow-lg text-center">
          <h1 className="text-xl font-serif font-bold text-text mb-sm">Connecting to Alexandryn</h1>
          <p className="text-sm text-text-3 mb-lg">
            The service is initializing or unreachable. Please wait a moment and try reconnecting.
          </p>
          <button
            type="button"
            onClick={() => void refetch()}
            className="inline-flex items-center justify-center rounded px-md py-sm text-sm font-ui font-medium border border-border bg-surface hover:bg-surface-2 text-text transition-colors"
          >
            Retry Connection
          </button>
        </div>
      </main>
    )
  }

  // If system is setup, require login
  const token = getAccessToken() || getRefreshToken()
  if (!token) {
    return <Navigate to="/login" state={{ from: location }} replace />
  }

  return <>{children}</>
}
