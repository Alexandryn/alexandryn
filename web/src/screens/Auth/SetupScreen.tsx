import React, { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '../../components/Button'
import { setupAdmin } from '../../data/auth'

export function SetupScreen() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const passwordRef = useRef<HTMLInputElement>(null)
  const confirmPasswordRef = useRef<HTMLInputElement>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      confirmPasswordRef.current?.focus()
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters long')
      passwordRef.current?.focus()
      return
    }

    setLoading(true)
    try {
      await setupAdmin({ username, email, password })
      navigate('/library', { replace: true })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Setup failed. Please check inputs.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-md">
      <div className="w-full max-w-md bg-surface p-xl rounded-lg border border-border shadow-lg">
        <div className="mb-lg text-center">
          <h1 className="text-2xl font-serif font-bold text-text mb-xs">Welcome to Alexandryn</h1>
          <p className="text-sm text-text-3">
            Create the master administrator account to initialize your library.
          </p>
        </div>

        {error && (
          <div
            id="setup-error"
            className="mb-md rounded border border-error bg-surface p-sm text-sm text-error"
            role="alert"
          >
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-md">
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="username">
              Admin Username
            </label>
            <input
              id="username"
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
              placeholder="e.g. librarian"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="email">
              Email Address
            </label>
            <input
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
              placeholder="admin@example.com"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="password">
              Password
            </label>
            <input
              ref={passwordRef}
              id="password"
              type="password"
              required
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              aria-invalid={error?.includes('Password') ? true : undefined}
              aria-describedby={error ? 'setup-error' : undefined}
              className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
              placeholder="••••••••"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="confirmPassword">
              Confirm Password
            </label>
            <input
              ref={confirmPasswordRef}
              id="confirmPassword"
              type="password"
              required
              minLength={8}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              aria-invalid={error === 'Passwords do not match' ? true : undefined}
              aria-describedby={error === 'Passwords do not match' ? 'setup-error' : undefined}
              className="px-md py-sm bg-background border border-border rounded text-text focus:outline-none focus:border-accent"
              placeholder="••••••••"
            />
          </div>

          <div className="mt-md">
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? 'Initializing...' : 'Initialize Alexandryn'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
