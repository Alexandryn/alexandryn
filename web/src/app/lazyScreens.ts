import { lazy } from 'react'

// Route-level code splitting: the Reader (and its epub
// vendor bundle), every host-only admin screen, and the auth/entry
// screens a signed-in user never revisits are loaded on demand, kept out
// of the initial chunk. The Library-first-paint screens stay eager and
// are imported directly in routes.tsx.

export const LoginScreen = lazy(() =>
  import('../screens/Auth/LoginScreen').then((m) => ({ default: m.LoginScreen })),
)
export const SetupScreen = lazy(() =>
  import('../screens/Auth/SetupScreen').then((m) => ({ default: m.SetupScreen })),
)
export const AcceptInviteScreen = lazy(() =>
  import('../screens/Auth/AcceptInviteScreen').then((m) => ({ default: m.AcceptInviteScreen })),
)
export const ForgotPasswordScreen = lazy(() =>
  import('../screens/Auth/ForgotPasswordScreen').then((m) => ({ default: m.ForgotPasswordScreen })),
)
export const ResetPasswordScreen = lazy(() =>
  import('../screens/Auth/ResetPasswordScreen').then((m) => ({ default: m.ResetPasswordScreen })),
)
export const ActivityScreen = lazy(() =>
  import('../screens/Activity').then((m) => ({ default: m.ActivityScreen })),
)
export const Import = lazy(() => import('../screens/Import').then((m) => ({ default: m.Import })))
export const Sources = lazy(() =>
  import('../screens/Sources').then((m) => ({ default: m.Sources })),
)
export const SourceDetail = lazy(() =>
  import('../screens/Sources').then((m) => ({ default: m.SourceDetail })),
)
export const LibraryManagement = lazy(() =>
  import('../screens/Libraries/LibraryManagement').then((m) => ({ default: m.LibraryManagement })),
)
export const Reader = lazy(() =>
  import('../screens/Reader').then((m) => ({ default: m.Reader })),
)
export const NetworkSettings = lazy(() =>
  import('../screens/Settings/NetworkSettings').then((m) => ({ default: m.NetworkSettings })),
)
export const DevicesSettings = lazy(() =>
  import('../screens/Settings/DevicesSettings').then((m) => ({ default: m.DevicesSettings })),
)
export const ConnectScreen = lazy(() =>
  import('../screens/Network/ConnectScreen').then((m) => ({ default: m.ConnectScreen })),
)
export const AccessScreen = lazy(() =>
  import('../screens/Network/AccessScreen').then((m) => ({ default: m.AccessScreen })),
)
