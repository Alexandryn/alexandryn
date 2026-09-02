import React, { useState } from 'react'
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

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters long')
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
    <div className="min-h-screen flex items-center justify-center bg-bg-canvas p-md">
      <div className="w-full max-w-md bg-bg-surface p-xl rounded-lg border border-border-subtle shadow-lg">
        <div className="mb-lg text-center">
          <h1 className="text-2xl font-serif font-bold text-text-primary mb-xs">Welcome to Alexandryn</h1>
          <p className="text-sm text-text-muted">Create the master administrator account to initialize your library.</p>
        </div>

        {error && (
          <div className="mb-md p-sm rounded bg-red-950/40 border border-red-800 text-red-300 text-sm" role="alert">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-md">
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="username">
              Admin Username
            </label>
            <input
              id="username"
              type="text"
              required
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary focus:outline-none focus:border-accent"
              placeholder="e.g. librarian"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="email">
              Email Address
            </label>
            <input
              id="email"
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary focus:outline-none focus:border-accent"
              placeholder="admin@example.com"
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
              minLength={8}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary focus:outline-none focus:border-accent"
              placeholder="••••••••"
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-secondary" htmlFor="confirmPassword">
              Confirm Password
            </label>
            <input
              id="confirmPassword"
              type="password"
              required
              minLength={8}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary focus:outline-none focus:border-accent"
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
