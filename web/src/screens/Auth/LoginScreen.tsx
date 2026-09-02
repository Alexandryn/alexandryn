import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../../components/Button'
import { login } from '../../data/auth'
import { MfaPromptModal } from './MfaPromptModal'

export function LoginScreen() {
  const navigate = useNavigate()
  const [emailOrUsername, setEmailOrUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [mfaTicket, setMfaTicket] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      const res = await login({ emailOrUsername, password })
      if (res.mfaRequired && res.mfaTicket) {
        setMfaTicket(res.mfaTicket)
      } else {
        navigate('/library', { replace: true })
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed. Please check your credentials.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-bg-canvas p-md">
      <div className="w-full max-w-md bg-bg-surface p-xl rounded-lg border border-border-subtle shadow-lg">
        <div className="mb-lg text-center">
          <h1 className="text-2xl font-serif font-bold text-text-primary mb-xs">Sign In</h1>
          <p className="text-sm text-text-muted">Access your Alexandryn library collection</p>
        </div>

        {error && (
          <div className="mb-md p-sm rounded bg-red-950/40 border border-red-800 text-red-300 text-sm" role="alert">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-md">
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="emailOrUsername">
              Email or Username
            </label>
            <input
              id="emailOrUsername"
              type="text"
              required
              value={emailOrUsername}
              onChange={(e) => setEmailOrUsername(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary focus:outline-none focus:border-accent"
              placeholder="username or email"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary focus:outline-none focus:border-accent"
              placeholder="••••••••"
            />
          </div>

          <div className="mt-md">
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? 'Signing in...' : 'Sign In'}
            </Button>
          </div>
        </form>
      </div>

      {mfaTicket && (
        <MfaPromptModal
          mfaTicket={mfaTicket}
          onSuccess={() => {
            setMfaTicket(null)
            navigate('/library', { replace: true })
          }}
          onCancel={() => setMfaTicket(null)}
        />
      )}
    </div>
  )
}
