import type { ReactNode } from 'react'
import { QueryClientProvider } from '@tanstack/react-query'
import { makeQueryClient } from './queryClient'

const queryClient = makeQueryClient()

/**
 * The app's data-layer boundary: everything below this is allowed to call
 * TanStack Query hooks, nothing calls fetch directly
 * (frontend-shell-and-routing.md FR-2). The router lives inside this in
 * main.tsx so loaders and components share one cache.
 */
export function AppProviders({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
