import React, { useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { Button } from '../../components/Button'
import { confirmPasswordReset } from '../../data/auth'

export function ResetPasswordScreen() {
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''

  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (password !== confirm) {
      setError('The two passwords do not match.')
      return
    }
    if (password.length < 8 || password.length > 128) {
      setError('Password must be between 8 and 128 characters.')
      return
    }

    setLoading(true)
    try {
      await confirmPasswordReset({ token, newPassword: password })
      navigate('/login', { replace: true, state: { passwordReset: true } })
    } catch {
      setError('This reset link is invalid or has expired. Request a new one.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center bg-background p-md">
      {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key (audit 0017 A-17-08) */}
      <div className="w-full max-w-[28rem] bg-surface p-xl rounded-lg border border-border shadow-lg">
        <div className="mb-lg text-center">
          <h1 className="text-2xl font-serif font-bold text-text mb-xs">Set a new password</h1>
          <p className="text-sm text-text-3">Choose a password you have not used here before.</p>
        </div>

        {!token ? (
          <div className="flex flex-col gap-md">
            <p
              className="rounded border border-error bg-surface p-sm text-sm text-error"
              role="alert"
            >
              This page needs a reset link. Request one from the sign-in page.
            </p>
            <Link to="/forgot-password" className="text-sm text-accent hover:underline">
              Request a reset link
            </Link>
          </div>
        ) : (
          <>
            {error && (
              <div
                className="mb-md rounded border border-error bg-surface p-sm text-sm text-error"
                role="alert"
              >
                {error}
              </div>
            )}

            <form onSubmit={handleSubmit} className="flex flex-col gap-md">
              <div className="flex flex-col gap-xs">
                <label className="text-sm font-medium text-text-2" htmlFor="password">
                  New password
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

              <div className="flex flex-col gap-xs">
                <label className="text-sm font-medium text-text-2" htmlFor="confirm">
                  Confirm new password
                </label>
                <input
                  id="confirm"
                  type="password"
                  required
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
                  placeholder="••••••••"
                />
              </div>

              <div className="mt-md">
                <Button type="submit" disabled={loading} className="w-full">
                  {loading ? 'Saving...' : 'Save new password'}
                </Button>
              </div>
            </form>
          </>
        )}
      </div>
    </main>
  )
}
