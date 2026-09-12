import { useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../../components/Button'
import { Spinner } from '../../components/Spinner/Spinner'
import { clearSession, getActiveLibraryId, getCurrentUser, logout } from '../../data/auth'
import { fetchLibraries, type Library } from '../../data/libraries'
import { useNetworkStatus } from '../../data/network'
import { useQuery } from '@tanstack/react-query'
import { getReachabilityDescription, getTLSDescription } from '../../lib/networkDescriptions'

export function AccessScreen() {
  const navigate = useNavigate()
  const {
    data: status,
    isLoading: statusLoading,
    isError: statusError,
    refetch: refetchStatus,
  } = useNetworkStatus()
  const user = getCurrentUser()

  const {
    data: librariesData,
    isError: librariesError,
    refetch: refetchLibraries,
  } = useQuery({
    queryKey: ['libraries'],
    queryFn: fetchLibraries,
  })

  // GET /api/v1/libraries is already scoped to the caller by the server
  // (a reader gets their memberships, an admin gets all). The client must
  // not re-derive access from the access token — that is the server's
  // decision, not something to parse out of a JWT here.
  const accessibleLibraries = useMemo(() => librariesData?.libraries ?? [], [librariesData])
  const libraries = accessibleLibraries
  const activeLibId = getActiveLibraryId() || (libraries[0]?.id ?? null)
  const activeLibrary = libraries.find((l) => l.id === activeLibId)

  const role = user?.role ?? 'reader'
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
    // max-w-[48rem] not max-w-3xl: --spacing-3xl collides with Tailwind's max-w-3xl key
    <div className="p-xl max-w-[48rem] mx-auto flex flex-col gap-2xl">
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
      {statusError && !status ? (
        <section
          aria-labelledby="connection-heading"
          className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-md shadow-sm"
        >
          <h2 id="connection-heading" className="text-lg font-ui font-medium text-text">
            Connection Details
          </h2>
          <div
            role="alert"
            className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error flex items-center justify-between gap-md"
          >
            <span>Could not load connection details.</span>
            <Button variant="secondary" size="sm" onClick={() => void refetchStatus()}>
              Retry
            </Button>
          </div>
        </section>
      ) : status ? (
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
      ) : null}

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

        {librariesError && accessibleLibraries.length === 0 ? (
          <div
            role="alert"
            className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error flex items-center justify-between gap-md"
          >
            <span>Could not load your libraries.</span>
            <Button variant="secondary" size="sm" onClick={() => void refetchLibraries()}>
              Retry
            </Button>
          </div>
        ) : accessibleLibraries.length === 0 ? (
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
