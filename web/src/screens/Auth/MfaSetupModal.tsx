import { useQuery } from '@tanstack/react-query'
import React, { useState } from 'react'
import { Button } from '../../components/Button'
import { confirmTOTP, setupTOTP } from '../../data/auth'

interface MfaSetupModalProps {
  isOpen: boolean
  onClose: () => void
  onSuccess: () => void
}

export function MfaSetupModal({ isOpen, onClose, onSuccess }: MfaSetupModalProps) {
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const { data, isLoading: initLoading, error: queryError } = useQuery({
    queryKey: ['totpSetup'],
    queryFn: setupTOTP,
    enabled: isOpen,
    staleTime: 0,
  })

  if (!isOpen) return null

  const handleConfirm = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      await confirmTOTP(code)
      onSuccess()
      onClose()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Invalid code. Please try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-md z-50 overflow-y-auto">
      <div className="w-full max-w-lg bg-bg-surface p-xl rounded-lg border border-border-subtle shadow-2xl my-md">
        <h2 className="text-xl font-serif font-bold text-text-primary mb-xs">Set Up Two-Factor Authentication</h2>
        <p className="text-sm text-text-muted mb-md">
          Protect your Alexandryn account with an authenticator app (Google Authenticator, Authy, 1Password, etc.).
        </p>

        {(error || (queryError instanceof Error ? queryError.message : null)) && (
          <div className="mb-md p-sm rounded bg-red-950/40 border border-red-800 text-red-300 text-sm" role="alert">
            {error || (queryError instanceof Error ? queryError.message : 'Failed to initialize 2FA setup')}
          </div>
        )}

        {initLoading ? (
          <div className="py-xl text-center text-text-muted">Generating 2FA keys...</div>
        ) : data ? (
          <form onSubmit={handleConfirm} className="flex flex-col gap-lg">
            <div className="p-md bg-bg-canvas border border-border-subtle rounded flex flex-col gap-xs">
              <span className="text-xs font-semibold text-text-muted uppercase">Secret Key</span>
              <code className="text-sm font-mono text-accent select-all break-all">{data.secret}</code>
            </div>

            <div className="flex flex-col gap-xs">
              <span className="text-xs font-semibold text-text-muted uppercase">Backup Recovery Codes</span>
              <p className="text-xs text-text-secondary">
                Save these emergency codes in a secure password manager. Each can be used once if you lose your authenticator device.
              </p>
              <div className="grid grid-cols-2 gap-xs p-sm bg-bg-canvas border border-border-subtle rounded font-mono text-xs text-text-primary">
                {data.recoveryCodes.map((c, i) => (
                  <div key={i} className="select-all">
                    {c}
                  </div>
                ))}
              </div>
            </div>

            <div className="flex flex-col gap-xs">
              <label className="text-sm font-medium text-text-secondary" htmlFor="confirmCode">
                Verification Code
              </label>
              <input
                id="confirmCode"
                type="text"
                required
                maxLength={6}
                value={code}
                onChange={(e) => setCode(e.target.value.trim())}
                className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary text-center text-lg tracking-widest focus:outline-none focus:border-accent"
                placeholder="000000"
              />
            </div>

            <div className="flex gap-sm">
              <Button type="button" variant="secondary" onClick={onClose} className="flex-1">
                Cancel
              </Button>
              <Button type="submit" disabled={loading || code.length !== 6} className="flex-1">
                {loading ? 'Activating...' : 'Activate 2FA'}
              </Button>
            </div>
          </form>
        ) : null}
      </div>
    </div>
  )
}
