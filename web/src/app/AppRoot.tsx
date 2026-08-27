import { RouterProvider } from 'react-router-dom'
import { CapabilityProvider } from './capability'
import { AppProviders } from './providers'
import { router } from './router'

/**
 * The whole app below the DOM root: the data layer (AppProviders), then
 * the capability context it feeds, then the router. Order matters —
 * CapabilityProvider issues a TanStack Query call, so it must sit inside
 * AppProviders.
 */
export function AppRoot() {
  return (
    <AppProviders>
      <CapabilityProvider>
        <RouterProvider router={router} />
      </CapabilityProvider>
    </AppProviders>
  )
}
