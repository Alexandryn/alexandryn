import { Navigate, type RouteObject } from 'react-router-dom'
import type { Capability } from '../data/bootstrap'
import { Library } from '../screens/Library'
import { NotFound } from '../screens/NotFound'
import { ParamPlaceholder, ScreenPlaceholder } from '../screens/ScreenPlaceholder'
import { RequireCapability } from './capability'
import { RouteError } from './RouteError'
import { AppShell } from './shell/AppShell'

// Route views are placeholders this phase — phase 06 onward fills them in
// inside the same shell (this spec's Non-goals). Only <NotFound> (FR-5)
// and, from T9, /library have real content.

function hostOnly(capability: Capability, title: string, note?: string) {
  return (
    <RequireCapability capability={capability}>
      <ScreenPlaceholder title={title} note={note} />
    </RequireCapability>
  )
}

const VIEWER_NOTE = 'Part of the network-access surface — built in a later phase.'

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
    errorElement: <RouteError />,
    children: [
      { index: true, element: <Navigate to="/library" replace /> },

      // Shared — render for the host window and a LAN browser alike.
      { path: 'library', element: <Library /> },
      { path: 'book/:id', element: <ParamPlaceholder title="Book" param="id" /> },
      { path: 'collections', element: <ScreenPlaceholder title="Collections" /> },
      { path: 'collections/:id', element: <ParamPlaceholder title="Collection" param="id" /> },
      { path: 'discover', element: <ScreenPlaceholder title="Discover" /> },
      { path: 'activity', element: <ScreenPlaceholder title="Activity" /> },
      { path: 'more', element: <ScreenPlaceholder title="More" /> },

      // Host-only (architecture-frontend.md FR-3) — gated on the capability value.
      { path: 'sources', element: hostOnly('sources', 'Sources') },
      { path: 'sources/:id', element: hostOnly('sources', 'Source') },
      { path: 'import', element: hostOnly('import', 'Import') },
      { path: 'settings', element: hostOnly('settings', 'Settings') },
      { path: 'system', element: hostOnly('system', 'System') },

      // Viewer surface — chrome deferred to phase 11/12/13 (design-conformance
      // finding 3); the routes are registered so the URLs resolve.
      { path: 'access', element: <ScreenPlaceholder title="Access" note={VIEWER_NOTE} /> },
      { path: 'connect', element: <ScreenPlaceholder title="Connect" note={VIEWER_NOTE} /> },
      {
        path: 'reader/:id',
        element: <ParamPlaceholder title="Reader" param="id" />,
      },

      { path: '*', element: <NotFound /> },
    ],
  },
]
