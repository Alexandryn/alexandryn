import React, { useState } from 'react'
import { Button } from '../../components/Button'
import { Modal } from '../../components/Modal'
import { verifyTOTP } from '../../data/auth'

interface MfaPromptModalProps {
  mfaTicket: string
  enrolmentGrant?: string
  onSuccess: () => void
  onCancel: () => void
}

export function MfaPromptModal({ mfaTicket, enrolmentGrant, onSuccess, onCancel }: MfaPromptModalProps) {
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
        await verifyTOTP({ mfaTicket, recoveryCode, enrolmentGrant })
      } else {
        await verifyTOTP({ mfaTicket, code, enrolmentGrant })
      }
      onSuccess()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'That code was not accepted. Check it and try again.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Modal
      open
      onOpenChange={(o) => {
        if (!o) onCancel()
      }}
      title="Two-factor authentication"
      description={
        useRecovery
          ? 'Enter one of your 10-character recovery codes.'
          : 'Enter the 6-digit code from your authenticator app.'
      }
      contentClassName="max-w-[24rem]" // --spacing-sm collision, audit 0017 A-17-08
    >
      {error && (
        <div className="mb-md mt-md rounded border border-error bg-surface p-sm text-sm text-error" role="alert">
          {error}
        </div>
      )}

      <form onSubmit={handleSubmit} className="mt-md flex flex-col gap-md">
        {!useRecovery ? (
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="totpCode">
              Authenticator code
            </label>
            <input
              id="totpCode"
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              // eslint-disable-next-line jsx-a11y/no-autofocus -- this modal exists only to capture one code; focus belongs on that field
              autoFocus
              required
              maxLength={6}
              value={code}
              onChange={(e) => setCode(e.target.value.trim())}
              className="rounded border border-border bg-background px-md py-sm text-center text-lg tracking-widest text-text focus:border-accent focus:outline-none"
              placeholder="000000"
            />
          </div>
        ) : (
          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="recoveryCode">
              Recovery code
            </label>
            <input
              id="recoveryCode"
              type="text"
              autoComplete="one-time-code"
              // eslint-disable-next-line jsx-a11y/no-autofocus -- this modal exists only to capture one code; focus belongs on that field
              autoFocus
              required
              value={recoveryCode}
              onChange={(e) => setRecoveryCode(e.target.value.trim())}
              className="rounded border border-border bg-background px-md py-sm text-center font-mono text-text focus:border-accent focus:outline-none"
              placeholder="XXXX-XXXX-XX"
            />
          </div>
        )}

        <div className="flex items-center justify-between text-xs">
          <button
            type="button"
            onClick={() => {
              setUseRecovery(!useRecovery)
              setError(null)
            }}
            className="text-accent hover:underline"
          >
            {useRecovery ? 'Use authenticator code' : 'Use a recovery code instead'}
          </button>
        </div>

        <div className="mt-sm flex gap-sm">
          <Button type="button" variant="secondary" onClick={onCancel} className="flex-1">
            Cancel
          </Button>
          <Button type="submit" disabled={loading} className="flex-1">
            {loading ? 'Verifying' : 'Verify'}
          </Button>
        </div>
      </form>
    </Modal>
  )
}
