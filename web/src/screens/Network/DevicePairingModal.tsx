import { useCallback, useEffect, useId, useState } from 'react'
import * as RadixDialog from '@radix-ui/react-dialog'
import { Button } from '../../components/Button'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  initiatePairing,
  useDeletePairing,
  type InitiatePairingResponse,
} from '../../data/network'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface DevicePairingModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function formatCountdown(totalSeconds: number): string {
  const m = Math.floor(totalSeconds / 60)
  const s = totalSeconds % 60
  return `${m}:${s < 10 ? '0' : ''}${s}`
}

export function DevicePairingModal({ open, onOpenChange }: DevicePairingModalProps) {
  const [session, setSession] = useState<InitiatePairingResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [secretRequired, setSecretRequired] = useState(false)
  const [secretInput, setSecretInput] = useState('')
  const [remainingSeconds, setRemainingSeconds] = useState(300)
  const [announcedRemaining, setAnnouncedRemaining] = useState('')

  const secretInputId = useId()
  const deleteMutation = useDeletePairing()

  const startInitiation = useCallback(
    async (secret?: string) => {
      setLoading(true)
      setError(null)
      try {
        const res = await initiatePairing(secret)
        setSession(res)
        setSecretRequired(false)
        const expiry = new Date(res.expiresAt).getTime()
        const diff = Math.max(0, Math.floor((expiry - Date.now()) / 1000))
        setRemainingSeconds(diff)
      } catch (err: unknown) {
        const msg = err instanceof Error ? err.message : 'Failed to initiate pairing'
        if (msg.toLowerCase().includes('secret') || (err as { status?: number }).status === 403) {
          setSecretRequired(true)
        }
        setError(msg)
      } finally {
        setLoading(false)
      }
    },
    [],
  )

  // Start initiate when modal opens; reset when it closes. Both branches
  // are scheduled rather than run directly: an effect body that sets state
  // synchronously (whether inline, like the reset below, or via a called
  // function like startInitiation) causes an extra cascading render
  // (react-hooks/set-state-in-effect) — scheduling makes the state update
  // happen outside the effect's own commit, at an imperceptible (0ms) delay.
  useEffect(() => {
    const id = setTimeout(() => {
      if (open) {
        if (!session) {
          void startInitiation()
        }
      } else {
        setSession(null)
        setError(null)
        setSecretRequired(false)
        setSecretInput('')
      }
    }, 0)
    return () => clearTimeout(id)
  }, [open, session, startInitiation])

  // Live countdown timer
  useEffect(() => {
    if (!open || !session) return

    const expiry = new Date(session.expiresAt).getTime()
    const updateCountdown = () => {
      const diff = Math.max(0, Math.floor((expiry - Date.now()) / 1000))
      setRemainingSeconds(diff)

      // Announce at 60s, 30s, 10s, and 0s only per FR criteria
      if (diff === 60) {
        setAnnouncedRemaining('60 seconds remaining')
      } else if (diff === 30) {
        setAnnouncedRemaining('30 seconds remaining')
      } else if (diff === 10) {
        setAnnouncedRemaining('10 seconds remaining')
      } else if (diff === 0) {
        setAnnouncedRemaining('This code expired')
      }
    }

    updateCountdown()
    const timer = setInterval(updateCountdown, 1000)
    return () => clearInterval(timer)
  }, [open, session])

  // QR code SVG matrix path. The `qrcode` library (~52 KB) is loaded on
  // demand — only when the pairing modal actually has a payload to render
  // — so it stays out of the NetworkSettings chunk (audit 0016 #163).
  const [qrSvgPath, setQrSvgPath] = useState<{ path: string; size: number } | null>(null)

  useEffect(() => {
    const payload = session?.payload
    let cancelled = false
    // Scheduled, not run inline: an effect that sets state synchronously
    // triggers an extra render (react-hooks/set-state-in-effect) — the
    // same pattern startInitiation's effect above uses.
    const id = setTimeout(() => {
      if (cancelled) return
      if (!payload) {
        setQrSvgPath(null)
        return
      }
      void import('qrcode')
        .then((mod) => {
          if (cancelled) return
          const qr = mod.default.create(payload, { errorCorrectionLevel: 'M' })
          const size = qr.modules.size
          let path = ''
          for (let r = 0; r < size; r++) {
            for (let c = 0; c < size; c++) {
              if (qr.modules.get(r, c)) {
                path += `M${c},${r}h1v1h-1z `
              }
            }
          }
          setQrSvgPath({ path, size })
        })
        .catch(() => {
          if (!cancelled) setQrSvgPath(null)
        })
    }, 0)
    return () => {
      cancelled = true
      clearTimeout(id)
    }
  }, [session?.payload])

  const [revokeError, setRevokeError] = useState<string | null>(null)

  const handleRevokeAndClose = useCallback(async () => {
    if (!session?.pairingId) {
      onOpenChange(false)
      return
    }
    setRevokeError(null)
    try {
      await deleteMutation.mutateAsync(session.pairingId)
      onOpenChange(false)
    } catch {
      // Do not close: a failed cancel leaves the pairing session live on
      // the server, so the user must know and be able to retry (audit
      // 0016 #142).
      setRevokeError(
        'Could not cancel the pairing session — it may still be active. Retry, or close and check your devices.',
      )
    }
  }, [session, deleteMutation, onOpenChange])

  const handleDone = useCallback(() => {
    // Done closes without revoking
    onOpenChange(false)
  }, [onOpenChange])

  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      <RadixDialog.Portal>
        <RadixDialog.Overlay
          className={cx(
            'fixed inset-0 bg-scrim/50 transition-opacity',
            'data-[state=closed]:opacity-0 data-[state=open]:opacity-100 motion-reduce:transition-none',
          )}
        />
        <RadixDialog.Content
          onEscapeKeyDown={(e) => {
            e.preventDefault()
            void handleRevokeAndClose()
          }}
          onPointerDownOutside={(e) => {
            // A backdrop click, like Escape and the corner ×, must revoke
            // the live pairing session and only close if that succeeds —
            // never bypass the failed-cancel notice (#142). Scoped to
            // pointer-down (not onInteractOutside) so a focus shift
            // during re-render cannot trigger it.
            e.preventDefault()
            void handleRevokeAndClose()
          }}
          className={cx(
            'fixed left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
            'rounded-lg bg-surface p-xl shadow-lg max-w-lg w-full flex flex-col gap-lg',
            'transition-opacity data-[state=closed]:opacity-0 data-[state=open]:opacity-100 motion-reduce:transition-none',
          )}
        >
          <div className="flex items-center justify-between">
            <RadixDialog.Title className="text-xl font-ui text-text font-medium">
              Pair a Device
            </RadixDialog.Title>
            {/* A plain button, not RadixDialog.Close: Close fires
                onOpenChange(false) unconditionally, which would unmount
                the modal before a failed revoke can be shown (#142).
                This routes through handleRevokeAndClose like Revoke and
                Escape. */}
            <button
              type="button"
              aria-label="Close"
              onClick={() => void handleRevokeAndClose()}
              className={cx('text-text-2 text-lg hover:text-text', FOCUS_RING)}
            >
              <span aria-hidden="true">×</span>
            </button>
          </div>

          <RadixDialog.Description className="text-sm text-text-2">
            Scan the QR code or enter the pairing code on your other device to connect.
          </RadixDialog.Description>

          {/* Polite live region for screen readers, announces only at 60/30/10/0s */}
          <div aria-live="polite" className="sr-only">
            {announcedRemaining}
          </div>

          {loading && (
            <div className="flex flex-col items-center justify-center p-3xl gap-md">
              <Spinner label="Creating pairing session" />
              <p className="text-xs text-text-2">Generating pairing code…</p>
            </div>
          )}

          {error && !secretRequired && (
            <div className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error">
              {error}
              <div className="mt-sm">
                <Button size="sm" variant="secondary" onClick={() => void startInitiation()}>
                  Try again
                </Button>
              </div>
            </div>
          )}

          {secretRequired && (
            <form
              onSubmit={(e) => {
                e.preventDefault()
                void startInitiation(secretInput)
              }}
              className="flex flex-col gap-md p-md border border-border rounded-md bg-surface-2"
            >
              <label htmlFor={secretInputId} className="text-sm font-medium text-text">
                Pairing Secret Required
              </label>
              <p className="text-xs text-text-2">
                This server requires a pairing secret to authorize new device connections.
              </p>
              <input
                id={secretInputId}
                type="password"
                value={secretInput}
                onChange={(e) => setSecretInput(e.target.value)}
                placeholder="Enter DEVICE_PAIRING_SECRET"
                className={cx(
                  'rounded-md border border-border bg-surface px-md py-xs text-sm text-text font-mono',
                  FOCUS_RING,
                )}
              />
              <Button type="submit" size="sm" variant="primary" disabled={!secretInput}>
                Authenticate & Pair
              </Button>
            </form>
          )}

          {session && (
            <div className="flex flex-col items-center gap-lg">
              {remainingSeconds === 0 ? (
                <div className="flex flex-col items-center gap-md p-2xl text-center">
                  <p className="text-md font-medium text-error">This code expired</p>
                  <p className="text-xs text-text-2">
                    Pairing sessions expire after 5 minutes for security.
                  </p>
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => void startInitiation(secretInput || undefined)}
                  >
                    Generate a new code
                  </Button>
                </div>
              ) : (
                <>
                  {/* QR code canvas / svg */}
                  {qrSvgPath && (
                    <div className="p-md bg-surface border border-border rounded-md shadow-sm">
                      <svg
                        viewBox={`0 0 ${qrSvgPath.size} ${qrSvgPath.size}`}
                        className="w-48 h-48"
                        role="img"
                        aria-label="Device pairing QR code"
                      >
                        <path d={qrSvgPath.path} fill="currentColor" className="text-text" />
                      </svg>
                    </div>
                  )}

                  {/* 8-char Crockford Base32 pairing code formatted XXXX-XXXX in mono */}
                  <div className="flex flex-col items-center gap-2xs">
                    <span className="text-xs uppercase tracking-8 text-text-3">Pairing Code</span>
                    <span
                      data-testid="pairing-code"
                      className="font-mono text-3xl font-bold tracking-7 text-accent bg-accent-soft/40 px-lg py-xs rounded-md border border-accent/20"
                    >
                      {session.code}
                    </span>
                  </div>

                  {/* Visual countdown */}
                  <div className="flex items-center gap-xs text-xs text-text-2">
                    <span>Expires in</span>
                    <span className="font-mono font-medium text-text">
                      {formatCountdown(remainingSeconds)}
                    </span>
                  </div>
                </>
              )}

              {revokeError && (
                <div
                  role="alert"
                  className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error"
                >
                  {revokeError}
                </div>
              )}

              {/* Actions: Revoke vs Done */}
              <div className="flex items-center justify-between w-full pt-md border-t border-border">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => void handleRevokeAndClose()}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending
                    ? 'Revoking…'
                    : revokeError
                      ? 'Retry revoke'
                      : 'Revoke'}
                </Button>
                <Button variant="secondary" size="sm" onClick={handleDone}>
                  Done
                </Button>
              </div>
            </div>
          )}
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  )
}
