import { useEffect, useId, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button } from '../../components/Button'
import { useVerifyPairing } from '../../data/network'
import { setPendingEnrolment } from '../../data/pendingEnrolment'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

function formatPairingCode(input: string): string {
  // Strip hyphens, whitespace, convert to uppercase
  const cleaned = input.toUpperCase().replace(/[^0-9A-Z]/g, '')
  if (cleaned.length <= 4) {
    return cleaned
  }
  return `${cleaned.slice(0, 4)}-${cleaned.slice(4, 8)}`
}

export function ConnectScreen() {
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const verifyMutation = useVerifyPairing()

  // ?c=<code> is read as the initial state directly (not set via an effect
  // after mount) — deriving it from a prop/URL on first render needs no
  // effect, and setting state synchronously inside one only costs an extra
  // render (react-hooks/set-state-in-effect).
  const [code, setCode] = useState(() => {
    const rawCode = searchParams.get('c')
    return rawCode ? formatPairingCode(rawCode) : ''
  })
  const [label, setLabel] = useState('')
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  const codeInputRef = useRef<HTMLInputElement>(null)
  const codeInputId = useId()
  const labelInputId = useId()

  // Strip ?c= from window history immediately once read, so it doesn't
  // linger in the URL/history after prefilling the form above.
  useEffect(() => {
    if (searchParams.get('c') && typeof window !== 'undefined') {
      const url = new URL(window.location.href)
      url.searchParams.delete('c')
      const newUrl = url.pathname + (url.search ? url.search : '')
      window.history.replaceState({}, '', newUrl)
    }
  }, [searchParams])

  // Imperative focus management (not the autoFocus JSX prop,
  // jsx-a11y/no-autofocus) is a legitimate effect: an external-system
  // (DOM) update, not a state derivation.
  useEffect(() => {
    codeInputRef.current?.focus()
  }, [])

  const handleCodeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const formatted = formatPairingCode(e.target.value)
    setCode(formatted)
    setErrorMessage(null)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setErrorMessage(null)

    const cleanCode = code.replace(/-/g, '').trim()
    if (cleanCode.length !== 8) {
      setErrorMessage('Pairing code must be 8 characters')
      codeInputRef.current?.focus()
      return
    }

    try {
      const res = await verifyMutation.mutateAsync({
        code: cleanCode,
        label: label.trim() || undefined,
      })

      // Carry enrolmentGrant + hostName to /login in router state, and
      // also mirror them into sessionStorage so a reload on /login does
      // not lose the in-progress enrolment (audit 0016 #157). Never the
      // URL — the grant is a bearer secret.
      setPendingEnrolment({ enrolmentGrant: res.enrolmentGrant, hostName: res.hostName })
      navigate('/login', {
        state: {
          enrolmentGrant: res.enrolmentGrant,
          hostName: res.hostName,
        },
        replace: true,
      })
    } catch (err: unknown) {
      const status = (err as { status?: number }).status
      if (status === 404) {
        setErrorMessage('pairing code not recognised')
      } else if (status === 429) {
        setErrorMessage('Too many attempts. Wait a minute and try again.')
      } else {
        const msg = err instanceof Error ? err.message : 'Failed to verify pairing code'
        setErrorMessage(msg)
      }
      codeInputRef.current?.focus()
    }
  }

  return (
    <div className="min-h-screen bg-background flex flex-col items-center justify-center p-xl">
      {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key (audit 0017 A-17-08) */}
      <div className="w-full max-w-[28rem] bg-surface border border-border rounded-lg shadow-sm p-2xl flex flex-col gap-xl">
        <div>
          <h1 className="text-2xl font-ui font-medium text-text">Connect</h1>
          <p className="text-sm text-text-2 mt-4xs">
            Enter the pairing code shown on your library host.
          </p>
        </div>

        {errorMessage && (
          <div
            role="alert"
            className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error"
          >
            {errorMessage}
          </div>
        )}

        <form onSubmit={handleSubmit} className="flex flex-col gap-lg">
          <div className="flex flex-col gap-xs">
            <label htmlFor={codeInputId} className="text-sm font-medium text-text">
              Pairing Code
            </label>
            <input
              ref={codeInputRef}
              id={codeInputId}
              type="text"
              autoComplete="off"
              autoCorrect="off"
              autoCapitalize="characters"
              spellCheck="false"
              placeholder="XXXX-XXXX"
              value={code}
              onChange={handleCodeChange}
              maxLength={9}
              className={cx(
                'rounded-md border border-border bg-surface px-md py-sm text-xl text-text font-mono font-bold tracking-7 text-center',
                FOCUS_RING,
              )}
            />
            <p className="text-xs text-text-2">
              8-character code displayed on your host screen.
            </p>
          </div>

          <div className="flex flex-col gap-xs">
            <label htmlFor={labelInputId} className="text-sm font-medium text-text">
              Name this device (optional)
            </label>
            <input
              id={labelInputId}
              type="text"
              placeholder="e.g. Living Room iPad"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              className={cx(
                'rounded-md border border-border bg-surface px-md py-xs text-sm text-text',
                FOCUS_RING,
              )}
            />
          </div>

          <Button
            type="submit"
            variant="primary"
            disabled={verifyMutation.isPending || code.replace(/-/g, '').length !== 8}
            className="w-full"
          >
            {verifyMutation.isPending ? 'Verifying…' : 'Continue'}
          </Button>
        </form>
      </div>
    </div>
  )
}
