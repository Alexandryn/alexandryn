import { useQuery } from '@tanstack/react-query'
import React, { useState } from 'react'
import { Button } from '../../components/Button'
import { Modal } from '../../components/Modal'
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

  const {
    data,
    isLoading: initLoading,
    error: queryError,
  } = useQuery({
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
      setError(err instanceof Error ? err.message : 'That code was not accepted. Check it and try again.')
    } finally {
      setLoading(false)
    }
  }

  const bannerMessage = error || (queryError instanceof Error ? queryError.message : null)

  return (
    <Modal
      open
      onOpenChange={(o) => {
        if (!o) onClose()
      }}
      title="Set up two-factor authentication"
      description="Protect your account with an authenticator app (Google Authenticator, Authy, 1Password, and similar)."
      contentClassName="max-w-[32rem]" // --spacing-lg collision
    >
      {bannerMessage && (
        <div className="mb-md mt-md rounded border border-error bg-surface p-sm text-sm text-error" role="alert">
          {bannerMessage || 'Could not start two-factor setup.'}
        </div>
      )}

      {initLoading ? (
        <div className="py-xl text-center text-text-3">Generating your keys</div>
      ) : data ? (
        <form onSubmit={handleConfirm} className="mt-md flex flex-col gap-lg">
          <div className="flex flex-col gap-xs rounded border border-border bg-background p-md">
            <span className="text-xs font-semibold uppercase text-text-3">Secret key</span>
            <code className="select-all break-all font-mono text-sm text-accent">{data.secret}</code>
          </div>

          <div className="flex flex-col gap-xs">
            <span className="text-xs font-semibold uppercase text-text-3">Backup recovery codes</span>
            <p className="text-xs text-text-2">
              Save these in a password manager. Each works once if you lose your authenticator device.
            </p>
            <div className="grid grid-cols-2 gap-xs rounded border border-border bg-background p-sm font-mono text-xs text-text">
              {data.recoveryCodes.map((c, i) => (
                <div key={i} className="select-all">
                  {c}
                </div>
              ))}
            </div>
          </div>

          <div className="flex flex-col gap-xs">
            <label className="text-sm font-medium text-text-2" htmlFor="confirmCode">
              Verification code
            </label>
            <input
              id="confirmCode"
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

          <div className="flex gap-sm">
            <Button type="button" variant="secondary" onClick={onClose} className="flex-1">
              Cancel
            </Button>
            <Button type="submit" disabled={loading || code.length !== 6} className="flex-1">
              {loading ? 'Activating' : 'Activate'}
            </Button>
          </div>
        </form>
      ) : null}
    </Modal>
  )
}
