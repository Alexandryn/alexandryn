import React, { useState } from 'react'
import { Link } from 'react-router-dom'
import { Button } from '../../components/Button'
import { requestPasswordReset } from '../../data/auth'

export function ForgotPasswordScreen() {
  const [email, setEmail] = useState('')
  const [submitted, setSubmitted] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await requestPasswordReset({ email })
      // The response is deliberately the same whether or not the address
      // is registered — do not reveal which.
      setSubmitted(true)
    } catch {
      setError('Could not send the reset email. Check your connection and try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-md">
      <div className="w-full max-w-md bg-surface p-xl rounded-lg border border-border shadow-lg">
        <div className="mb-lg text-center">
          <h1 className="text-2xl font-serif font-bold text-text mb-xs">Reset your password</h1>
          <p className="text-sm text-text-3">
            Enter your account email and we will send a link to set a new password.
          </p>
        </div>

        {submitted ? (
          <div className="flex flex-col gap-md">
            <p className="text-sm text-text-2" role="status">
              If that email is registered, a password reset link is on its way. The link expires
              after a short time.
            </p>
            <Link to="/login" className="text-sm text-accent hover:underline">
              Back to sign in
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
                <label className="text-sm font-medium text-text-2" htmlFor="email">
                  Email
                </label>
                <input
                  id="email"
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
                  placeholder="you@example.com"
                />
              </div>

              <div className="mt-md">
                <Button type="submit" disabled={loading} className="w-full">
                  {loading ? 'Sending...' : 'Send reset link'}
                </Button>
              </div>
            </form>

            <div className="mt-md text-center">
              <Link to="/login" className="text-sm text-accent hover:underline">
                Back to sign in
              </Link>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
