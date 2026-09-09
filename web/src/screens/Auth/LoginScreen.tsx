import React, { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { Button } from '../../components/Button'
import { login } from '../../data/auth'
import { clearPendingEnrolment, getPendingEnrolment } from '../../data/pendingEnrolment'
import { MfaPromptModal } from './MfaPromptModal'

export function LoginScreen() {
  const navigate = useNavigate()
  const location = useLocation()
  const routerState = location.state as
    | {
        enrolmentGrant?: string
        hostName?: string
        from?: { pathname?: string; search?: string }
        passwordReset?: boolean
      }
    | null
  // Router state is lost on a reload; fall back to the sessionStorage
  // mirror ConnectScreen wrote (audit 0016 #157).
  const enrolmentGrant = routerState?.enrolmentGrant ?? getPendingEnrolment()?.enrolmentGrant

  // Where to land after a successful sign-in: an explicit router `from`
  // (RequireAuth), a ?next= / ?returnTo= query param (the global 401
  // redirect and external links), or the library (audit 0016 #91, #95,
  // #161).
  const fromState = routerState?.from
  const search = new URLSearchParams(location.search)
  const nextParam = search.get('next') ?? search.get('returnTo')
  const returnTo =
    (fromState?.pathname ? fromState.pathname + (fromState.search ?? '') : null) ??
    nextParam ??
    '/library'

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
      const res = await login({ emailOrUsername, password, enrolmentGrant })
      if (res.mfaRequired && res.mfaTicket) {
        setMfaTicket(res.mfaTicket)
      } else {
        clearPendingEnrolment()
        navigate(returnTo, { replace: true })
      }
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed. Please check your credentials.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-md">
      <div className="w-full max-w-md bg-surface p-xl rounded-lg border border-border shadow-lg">
        <div className="mb-lg text-center">
          <h1 className="text-2xl font-serif font-bold text-text mb-xs">Sign In</h1>
          <p className="text-sm text-text-3">Access your Alexandryn library collection</p>
        </div>

        {routerState?.passwordReset && !error && (
          <div className="mb-md rounded border border-border bg-surface p-sm text-sm text-text-2" role="status">
            Your password has been changed. Sign in with your new password.
          </div>
        )}

        {error && (
          <div className="mb-md rounded border border-error bg-surface p-sm text-sm text-error" role="alert">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-md">
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="emailOrUsername">
              Email or Username
            </label>
            <input
              id="emailOrUsername"
              type="text"
              required
              value={emailOrUsername}
              onChange={(e) => setEmailOrUsername(e.target.value)}
              className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
              placeholder="username or email"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
              placeholder="••••••••"
            />
          </div>

          <div className="mt-md">
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? 'Signing in...' : 'Sign In'}
            </Button>
          </div>
        </form>

        <div className="mt-md text-center">
          <Link to="/forgot-password" className="text-sm text-accent hover:underline">
            Forgot your password?
          </Link>
        </div>
      </div>

      {mfaTicket && (
        <MfaPromptModal
          mfaTicket={mfaTicket}
          enrolmentGrant={enrolmentGrant}
          onSuccess={() => {
            setMfaTicket(null)
            clearPendingEnrolment()
            navigate(returnTo, { replace: true })
          }}
          onCancel={() => setMfaTicket(null)}
        />
      )}
    </div>
  )
}
