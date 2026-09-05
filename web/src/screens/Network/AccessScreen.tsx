import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../../components/Button'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  clearSession,
  getAccessToken,
  getActiveLibraryId,
  getCurrentUser,
  logout,
} from '../../data/auth'
import { fetchLibraries, type Library } from '../../data/libraries'
import { useNetworkStatus } from '../../data/network'
import { useQuery } from '@tanstack/react-query'
import { getReachabilityDescription, getTLSDescription } from '../Settings/NetworkSettings'

function decodeJwtPayload(token: string | null): Record<string, unknown> | null {
  if (!token) return null
  try {
    const parts = token.split('.')
    const payloadPart = parts[1]
    if (!payloadPart) return null
    const base64 = payloadPart.replace(/-/g, '+').replace(/_/g, '/')
    const json = atob(base64)
    return JSON.parse(json) as Record<string, unknown>
  } catch {
    return null
  }
}

export function AccessScreen() {
  const navigate = useNavigate()
  const { data: status, isLoading: statusLoading } = useNetworkStatus()
  const user = getCurrentUser()
  const token = getAccessToken()

  const claims = useMemo(() => decodeJwtPayload(token), [token])

  const { data: librariesData } = useQuery({
    queryKey: ['libraries'],
    queryFn: fetchLibraries,
  })

  const libraries = librariesData?.libraries || []
  const activeLibId = getActiveLibraryId() || (libraries[0]?.id ?? null)
  const activeLibrary = libraries.find((l) => l.id === activeLibId)

  // Accessible library names: match claims.libraries (IDs) against fetched libraries
  const libraryIdsInToken = useMemo(() => {
    if (claims && Array.isArray(claims.libraries)) {
      return claims.libraries as string[]
    }
    return []
  }, [claims])

  const accessibleLibraries = useMemo(() => {
    if (libraryIdsInToken.length > 0) {
      return libraries.filter((l) => libraryIdsInToken.includes(l.id))
    }
    return libraries
  }, [libraries, libraryIdsInToken])

  const role = user?.role || (claims?.role as string) || 'reader'
  const isAdmin = role === 'admin'
  const allowUploads = isAdmin || (activeLibrary?.allowReaderUploads ?? false)

  const handleSignOut = async () => {
    try {
      await logout()
    } catch {
      clearSession()
    }
    navigate('/login', { replace: true })
  }

  if (statusLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-3xl gap-md min-h-screen">
        <Spinner label="Loading connection details" />
      </div>
    )
  }

  return (
    <div className="p-xl max-w-3xl mx-auto flex flex-col gap-2xl">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-ui font-medium text-text">Access</h1>
          <p className="text-sm text-text-2 mt-4xs">
            Review your current connection details and device permissions.
          </p>
        </div>
        <Button variant="secondary" size="sm" onClick={handleSignOut}>
          Sign out of this browser
        </Button>
      </div>

      {/* Network Status Card */}
      {status && (
        <section aria-labelledby="connection-heading" className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-lg shadow-sm">
          <h2 id="connection-heading" className="text-lg font-ui font-medium text-text">
            Connection Details
          </h2>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-lg text-sm">
            <div className="flex flex-col gap-4xs">
              <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Reachability</span>
              <span data-testid="access-reachability" className="font-medium text-text">
                {getReachabilityDescription(status.reachability)}
              </span>
            </div>

            <div className="flex flex-col gap-4xs">
              <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Server Address</span>
              <span data-testid="access-address" className="font-mono text-text">
                {status.address}
              </span>
            </div>

            <div className="flex flex-col gap-4xs md:col-span-2">
              <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Security & Transport</span>
              <span data-testid="access-tls" className="text-text-2 leading-relaxed">
                {getTLSDescription(status.tlsMode)}
              </span>
            </div>
          </div>
        </section>
      )}

      {/* Capabilities & Permissions Card */}
      <section aria-labelledby="permissions-heading" className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-lg shadow-sm">
        <div className="flex items-center justify-between">
          <h2 id="permissions-heading" className="text-lg font-ui font-medium text-text">
            Device Permissions
          </h2>
          <span className="text-xs uppercase tracking-6 px-sm py-4xs rounded-md bg-accent-soft text-accent font-medium">
            Role: {role}
          </span>
        </div>

        <div className="flex flex-col gap-md text-sm">
          <p className="text-xs text-text-2">
            This session is authenticated as <strong className="text-text font-medium">{user?.username || 'Current User'}</strong>. Your permissions on this network connection:
          </p>

          <ul className="flex flex-col gap-xs list-disc list-inside text-text-2">
            <li className="text-text">Browse catalog and search books</li>
            <li className="text-text">Read books in the web reader and sync reading progress</li>
            <li className="text-text">Manage personal bookmarks and highlights</li>
            {allowUploads ? (
              <li className="text-text" data-testid="capability-upload">
                Upload new books to the active library
              </li>
            ) : (
              <li className="text-text-3 italic" data-testid="capability-upload-disabled">
                Upload: disabled by library policy for readers
              </li>
            )}
            {isAdmin && (
              <>
                <li className="text-text">Manage libraries, members, and invitations</li>
                <li className="text-text">Configure sources and import pipelines</li>
                <li className="text-text">Initiate pairing for other devices</li>
              </>
            )}
          </ul>
        </div>
      </section>

      {/* Accessible Libraries Card */}
      <section aria-labelledby="libraries-heading" className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-lg shadow-sm">
        <h2 id="libraries-heading" className="text-lg font-ui font-medium text-text">
          Accessible Libraries
        </h2>

        {accessibleLibraries.length === 0 ? (
          <p className="text-sm text-text-2">No libraries assigned to this account.</p>
        ) : (
          <div className="flex flex-col gap-sm">
            {accessibleLibraries.map((lib: Library) => (
              <div
                key={lib.id}
                className="flex items-center justify-between p-md rounded-md bg-surface-2 border border-border"
              >
                <div>
                  <span className="font-medium text-sm text-text">{lib.name}</span>
                  {lib.description && (
                    <p className="text-xs text-text-2 mt-4xs">{lib.description}</p>
                  )}
                </div>
                {lib.id === activeLibId && (
                  <span className="text-xs text-accent font-medium px-sm py-4xs rounded bg-accent-soft">
                    Active
                  </span>
                )}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
