import { Navigate, type RouteObject } from 'react-router-dom'
import type { Capability } from '../data/bootstrap'
import { Collections } from '../screens/Collections'
import { CollectionDetail } from '../screens/CollectionDetail'
import { Discover, DiscoverWorkDetail } from '../screens/Discover'
import { Library } from '../screens/Library'
import { NotFound } from '../screens/NotFound'
import { ParamPlaceholder, ScreenPlaceholder } from '../screens/ScreenPlaceholder'
import { Sources, SourceDetail } from '../screens/Sources'
import { WorkDetail } from '../screens/WorkDetail'

import { RequireCapability } from './capability'
import { RouteError } from './RouteError'
import { AppShell } from './shell/AppShell'

// Route views are placeholders this phase — phase 06 onward fills them in
// inside the same shell (this spec's Non-goals). Only <NotFound> (FR-5),
// /library, /book/:id, and /discover have real content.
//
// Host-only gating (architecture-frontend.md FR-3) is declared here, in
// one place — the `hostOnly()` wrapper below. The sidebar shows every
// link unconditionally for now (the mock grants all capabilities); phase
// 12/13 is where a denied capability hides or disables its nav link.

function hostOnly(capability: Capability, title: string, note?: string) {
  return (
    <RequireCapability capability={capability}>
      <ScreenPlaceholder title={title} note={note} />
    </RequireCapability>
  )
}

const VIEWER_NOTE = 'Part of the network-access surface — built in a later phase.'

const shellChildren: RouteObject[] = [
  { index: true, element: <Navigate to="/library" replace /> },

  // Shared — render for the host window and a LAN browser alike.
  { path: 'library', element: <Library /> },
  { path: 'book/:id', element: <WorkDetail /> },

  { path: 'collections', element: <Collections /> },
  { path: 'collections/:id', element: <CollectionDetail /> },
  { path: 'collection/:id', element: <CollectionDetail /> },
  { path: 'discover', element: <Discover /> },
  { path: 'discover/works/:openLibraryId', element: <DiscoverWorkDetail /> },
  { path: 'activity', element: <ScreenPlaceholder title="Activity" /> },
  { path: 'more', element: <ScreenPlaceholder title="More" /> },

  // Host-only.
  {
    path: 'sources',
    element: (
      <RequireCapability capability="sources">
        <Sources />
      </RequireCapability>
    ),
  },
  {
    path: 'sources/:id',
    element: (
      <RequireCapability capability="sources">
        <SourceDetail />
      </RequireCapability>
    ),
  },
  { path: 'import', element: hostOnly('import', 'Import') },
  { path: 'settings', element: hostOnly('settings', 'Settings') },
  { path: 'system', element: hostOnly('system', 'System') },

  // Viewer surface — chrome deferred to phase 11/12/13 (design-conformance
  // finding 3); the routes are registered so the URLs resolve.
  { path: 'access', element: <ScreenPlaceholder title="Access" note={VIEWER_NOTE} /> },
  { path: 'connect', element: <ScreenPlaceholder title="Connect" note={VIEWER_NOTE} /> },
  { path: 'reader/:id', element: <ParamPlaceholder title="Reader" param="id" /> },
  { path: 'read/:workId/:editionId', element: <ParamPlaceholder title="Reader" param="workId" /> },

  { path: '*', element: <NotFound /> },
]

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <AppShell />,
    children: [
      // A pathless layout child carries the errorElement, so a render
      // error in any screen shows <RouteError> inside the shell (nav
      // still reachable), not in place of it (code review finding 2).
      { errorElement: <RouteError />, children: shellChildren },
    ],
  },
]
