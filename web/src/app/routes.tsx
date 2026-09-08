import { Navigate, type RouteObject } from 'react-router-dom'
import type { Capability } from '../data/bootstrap'

// Eager — on the Library-first-paint path or cheap enough that a second
// network round-trip would cost more than it saves.
import { Library } from '../screens/Library'
import { WorkDetail } from '../screens/WorkDetail'
import { Collections } from '../screens/Collections'
import { CollectionDetail } from '../screens/CollectionDetail'
import { Discover, DiscoverWorkDetail } from '../screens/Discover'
import { NotFound } from '../screens/NotFound'
import { ParamPlaceholder, ScreenPlaceholder } from '../screens/ScreenPlaceholder'
import { SettingsIndex } from '../screens/Settings/SettingsIndex'
import { MoreScreen } from '../screens/shell/MoreScreen'

// Lazy route components — see lazyScreens.ts (audit 0016 issue 100).
import {
  AccessScreen,
  AcceptInviteScreen,
  ActivityScreen,
  ConnectScreen,
  DevicesSettings,
  Import,
  LibraryManagement,
  LoginScreen,
  NetworkSettings,
  Reader,
  SetupScreen,
  SourceDetail,
  Sources,
} from './lazyScreens'

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
  { path: 'activity', element: <ActivityScreen /> },
  { path: 'more', element: <MoreScreen /> },

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
  { path: 'settings', element: <SettingsIndex /> },
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
    children: [{ errorElement: <RouteError />, children: shellChildren }],
  },
]
