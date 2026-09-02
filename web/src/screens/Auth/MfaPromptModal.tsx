import React, { useState } from 'react'
import { Button } from '../../components/Button'
import { verifyTOTP } from '../../data/auth'

interface MfaPromptModalProps {
  mfaTicket: string
  onSuccess: () => void
  onCancel: () => void
}

export function MfaPromptModal({ mfaTicket, onSuccess, onCancel }: MfaPromptModalProps) {
  const [code, setCode] = useState('')
  const [recoveryCode, setRecoveryCode] = useState('')
  const [useRecovery, setUseRecovery] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setLoading(true)

    try {
      if (useRecovery) {
        await verifyTOTP({ mfaTicket, recoveryCode })
      } else {
        await verifyTOTP({ mfaTicket, code })
      }
      onSuccess()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Verification failed. Please check the code.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center p-md z-50">
      <div className="w-full max-w-sm bg-bg-surface p-xl rounded-lg border border-border-subtle shadow-2xl">
        <h2 className="text-xl font-serif font-bold text-text-primary mb-xs">Two-Factor Authentication</h2>
        <p className="text-sm text-text-muted mb-md">
          {useRecovery
            ? 'Enter one of your 10-character recovery codes.'
            : 'Enter the 6-digit verification code from your authenticator app.'}
        </p>

        {error && (
          <div className="mb-md p-sm rounded bg-red-950/40 border border-red-800 text-red-300 text-sm" role="alert">
            {error}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-md">
          {!useRecovery ? (
            <div className="flex flex-col gap-xs">
              <label className="text-sm font-medium text-text-secondary" htmlFor="totpCode">
                Authenticator Code
              </label>
              <input
                id="totpCode"
                type="text"
                required
                maxLength={6}
                value={code}
                onChange={(e) => setCode(e.target.value.trim())}
                className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary text-center text-lg tracking-widest focus:outline-none focus:border-accent"
                placeholder="000000"
              />
            </div>
          ) : (
            <div className="flex flex-col gap-xs">
              <label className="text-sm font-medium text-text-secondary" htmlFor="recoveryCode">
                Recovery Code
              </label>
              <input
                id="recoveryCode"
                type="text"
                required
                value={recoveryCode}
                onChange={(e) => setRecoveryCode(e.target.value.trim())}
                className="px-md py-sm bg-bg-canvas border border-border-subtle rounded text-text-primary text-center font-mono focus:outline-none focus:border-accent"
                placeholder="XXXX-XXXX-XX"
              />
            </div>
          )}

          <div className="flex justify-between items-center text-xs">
            <button
              type="button"
              onClick={() => {
                setUseRecovery(!useRecovery)
                setError(null)
              }}
              className="text-accent hover:underline"
            >
              {useRecovery ? 'Use authenticator code' : 'Use recovery code instead'}
            </button>
          </div>

          <div className="flex gap-sm mt-sm">
            <Button type="button" variant="secondary" onClick={onCancel} className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading} className="flex-1">
              {loading ? 'Verifying...' : 'Verify'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
