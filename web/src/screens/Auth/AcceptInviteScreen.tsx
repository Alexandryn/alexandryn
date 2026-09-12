import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useLocation, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../../components/Button'
import { getAccessToken } from '../../data/auth'
import { switchActiveLibrary } from '../../data/activeLibrary'
import { acceptLibraryInvitation } from '../../data/libraries'

export function AcceptInviteScreen() {
  const { token } = useParams<{ token: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const isAuthenticated = !!getAccessToken()

  const handleAccept = async () => {
    if (!token) return
    setError(null)
    setLoading(true)

    try {
      const res = await acceptLibraryInvitation(token)
      // A new library was joined: refresh its list too, then switch to it
      // and drop any cache scoped to the old active library.
      void queryClient.invalidateQueries({ queryKey: ['libraries'] })
      switchActiveLibrary(queryClient, res.libraryId)
      navigate('/library', { replace: true })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to accept invitation. The link may have expired.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center bg-background p-md">
      {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key */}
      <div className="w-full max-w-[28rem] bg-surface p-xl rounded-lg border border-border shadow-lg text-center">
        <h1 className="text-2xl font-serif font-bold text-text mb-xs">Library invitation</h1>
        <p className="text-sm text-text-3 mb-lg">You have been invited to join an Alexandryn library namespace.</p>

        {error && (
          <div className="mb-md rounded border border-error bg-surface p-sm text-sm text-error" role="alert">
            {error}
          </div>
        )}

        {!isAuthenticated ? (
          <div className="flex flex-col gap-md">
            <p className="text-sm text-text-2">Please sign in to your Alexandryn account to accept this invitation.</p>
            {/* Preserve the invite URL so login returns here and the token
                is not lost. */}
            <Button onClick={() => navigate('/login', { state: { from: location } })}>Sign in</Button>
          </div>
        ) : (
          <div className="flex flex-col gap-md">
            <Button onClick={handleAccept} disabled={loading} className="w-full">
              {loading ? 'Joining...' : 'Accept & Join Library'}
            </Button>
          </div>
        )}
      </div>
    </main>
  )
}
