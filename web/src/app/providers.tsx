import { useState, type ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { makeQueryClient } from './queryClient'

/**
 * The app's data-layer boundary: everything below this is allowed to call
 * TanStack Query hooks, nothing calls fetch directly. The router lives
 * inside this in main.tsx so loaders and components share one cache.
 * Uses a per-instance QueryClient rather than a module singleton.
 */
export function AppProviders({ children, client }: { children: ReactNode; client?: QueryClient }) {
  const [queryClient] = useState(() => client ?? makeQueryClient())
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
