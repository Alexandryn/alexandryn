import React, { useContext, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { QueryClientContext } from '@tanstack/react-query'
import { Button } from '../../components/Button'
import { Input } from '../../components/Input'
import { AlexAvatar } from '../../components/Mascot'
import { setupAdmin } from '../../data/auth'
import { ApiError } from '../../data/http'

export function SetupScreen() {
  const navigate = useNavigate()
  const queryClient = useContext(QueryClientContext)
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
      setError('Passwords do not match. Check both fields and try again.')
      confirmPasswordRef.current?.focus()
      return
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters long. Choose a longer password and try again.')
      passwordRef.current?.focus()
      return
    }

    setLoading(true)
    try {
       await setupAdmin({ username, email, password })
       queryClient?.clear()
       navigate('/library', { replace: true })
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 409 || err.message === 'system already initialized') {
          setError('Alexandryn has already been initialized. Sign in with your administrator account.')
        } else if (err.status === 403 || err.code === 'Forbidden' || err.message.includes('origin')) {
          setError(
            'Setup request was blocked by origin validation. Connect through an allowed address or configure CORS_ALLOWED_ORIGINS.',
          )
        } else if (err.status === 400 || err.code === 'InvalidInput') {
          setError(
            err.message && err.message !== 'malformed request payload'
              ? err.message
              : 'Invalid account details. Check username, email, and password requirements, then try again.',
          )
        } else {
          setError(err.message || 'Could not initialize Alexandryn. Check your connection and try again.')
        }
      } else if (err instanceof Error) {
        setError(err.message)
      } else {
        setError('Could not initialize Alexandryn. Check your connection and try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center bg-background p-md">
      {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key */}
      <div className="w-full max-w-[28rem] bg-surface p-xl rounded-lg border border-border shadow-lg">
        <div className="mb-lg text-center">
          <AlexAvatar size="lg" className="mx-auto mb-md" />
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

        {/* Shared Input: same verified focus-visible outline every other field in the app uses. Error state stays the single shared #setup-error banner above, not Input's own per-field error prop, so aria-invalid/aria-describedby are passed through directly. */}
        <form onSubmit={handleSubmit} className="flex flex-col gap-md">
          <Input
            label="Admin Username"
            id="username"
            type="text"
            required
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="e.g. librarian"
          />

          <Input
            label="Email Address"
            id="email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="admin@example.com"
          />

          <Input
            ref={passwordRef}
            label="Password"
            id="password"
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            aria-invalid={error?.includes('Password') ? true : undefined}
            aria-describedby={error ? 'setup-error' : undefined}
            placeholder="••••••••"
          />

          <Input
            ref={confirmPasswordRef}
            label="Confirm Password"
            id="confirmPassword"
            type="password"
            required
            minLength={8}
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            aria-invalid={error?.startsWith('Passwords do not match') ? true : undefined}
            aria-describedby={error?.startsWith('Passwords do not match') ? 'setup-error' : undefined}
            placeholder="••••••••"
          />

          <div className="mt-md">
            <Button type="submit" disabled={loading} className="w-full">
              {loading ? 'Initializing...' : 'Initialize Alexandryn'}
            </Button>
          </div>
        </form>
      </div>
    </main>
  )
}
