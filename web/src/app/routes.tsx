import { Navigate, type RouteObject } from 'react-router-dom'
import type { Capability } from '../data/bootstrap'
import { AcceptInviteScreen } from '../screens/Auth/AcceptInviteScreen'
import { LoginScreen } from '../screens/Auth/LoginScreen'
import { SetupScreen } from '../screens/Auth/SetupScreen'
import { CollectionDetail } from '../screens/CollectionDetail'
import { Collections } from '../screens/Collections'
import { Discover, DiscoverWorkDetail } from '../screens/Discover'
import { Import } from '../screens/Import'
import { Library } from '../screens/Library'
import { LibraryManagement } from '../screens/Libraries/LibraryManagement'
import { NotFound } from '../screens/NotFound'
import { Reader } from '../screens/Reader'
import { ParamPlaceholder, ScreenPlaceholder } from '../screens/ScreenPlaceholder'
import { Sources, SourceDetail } from '../screens/Sources'
import { WorkDetail } from '../screens/WorkDetail'
import { NetworkSettings } from '../screens/Settings/NetworkSettings'
import { DevicesSettings } from '../screens/Settings/DevicesSettings'
import { ConnectScreen } from '../screens/Network/ConnectScreen'
import { AccessScreen } from '../screens/Network/AccessScreen'

import { RequireAuth } from './auth/RequireAuth'
import { RequireCapability } from './capability'
import { RouteError } from './RouteError'
import { AppShell } from './shell/AppShell'

function hostOnly(capability: Capability, title: string, note?: string) {
  return (
    <RequireCapability capability={capability}>
      <ScreenPlaceholder title={title} note={note} />
    </RequireCapability>
  )
}

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

  // Multi-library administration (Phase 12)
  {
    path: 'libraries',
    element: (
      <RequireCapability capability="settings">
        <LibraryManagement />
      </RequireCapability>
    ),
  },

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
  {
    path: 'import',
    element: (
      <RequireCapability capability="import">
        <Import />
      </RequireCapability>
    ),
  },
  {
    path: 'settings/network',
    element: (
      <RequireCapability capability="network">
        <NetworkSettings />
      </RequireCapability>
    ),
  },
  {
    path: 'settings/devices',
    element: (
      <RequireCapability capability="settings">
        <DevicesSettings />
      </RequireCapability>
    ),
  },
  { path: 'network', element: <Navigate to="/settings/network" replace /> },
  { path: 'devices', element: <Navigate to="/settings/devices" replace /> },
  { path: 'settings', element: hostOnly('settings', 'Settings') },
  { path: 'system', element: hostOnly('system', 'System') },

  // Viewer surface (Phase 13)
  { path: 'access', element: <AccessScreen /> },
  { path: 'connect', element: <ConnectScreen /> },
  { path: 'reader/:id', element: <ParamPlaceholder title="Reader" param="id" /> },
  { path: 'read/:workId/:editionId', element: <Reader /> },

  { path: '*', element: <NotFound /> },
]

export const routes: RouteObject[] = [
  // Public unauthenticated routes
  { path: '/setup', element: <SetupScreen /> },
  { path: '/login', element: <LoginScreen /> },
  { path: '/invite/:token', element: <AcceptInviteScreen /> },
  { path: '/connect', element: <ConnectScreen /> },

  // Authenticated shell
  {
    path: '/',
    element: (
      <RequireAuth>
        <AppShell />
      </RequireAuth>
    ),
    children: [
      { errorElement: <RouteError />, children: shellChildren },
    ],
  },
]

