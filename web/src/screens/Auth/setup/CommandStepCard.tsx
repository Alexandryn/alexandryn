import { useState } from 'react'
import { Button } from '../../../components/Button'
import { StatusPill, type StatusTone } from '../../../components/StatusPill/StatusPill'
import { cx } from '../../../lib/cx'

export type CommandStepStatus = 'pending' | 'executing' | 'verified' | 'failed'

export interface CommandStepCardProps {
  title: string
  rationale: string
  target: 'Host Terminal' | 'Docker Terminal' | 'Browser' | string
  command: string
  status: CommandStepStatus
  onVerify: () => Promise<void> | void
  verifying?: boolean
  errorMessage?: string
  fallbackText?: string
  fallbackLabel?: string
  onFallback?: () => void
  className?: string
}

export function CommandStepCard({
  title,
  rationale,
  target,
  command,
  status,
  onVerify,
  verifying = false,
  errorMessage,
  fallbackText,
  fallbackLabel = 'Use Alternative Fallback',
  onFallback,
  className,
}: CommandStepCardProps) {
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard) {
        await navigator.clipboard.writeText(command)
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }
    } catch {
      // Clipboard write failed; non-fatal
    }
  }

  const isBusy = verifying || status === 'executing'

  const toneMap: Record<CommandStepStatus, StatusTone> = {
    pending: 'neutral',
    executing: 'accent',
    verified: 'success',
    failed: 'error',
  }

  const labelMap: Record<CommandStepStatus, string> = {
    pending: 'Pending',
    executing: 'Verifying…',
    verified: 'Verified',
    failed: 'Verification Failed',
  }

  return (
    <div
      className={cx(
        'rounded-lg border border-border bg-surface p-md text-text flex flex-col gap-sm',
        className,
      )}
      data-testid="command-step-card"
    >
      <div className="flex items-start justify-between gap-sm flex-wrap">
        <div>
          <div className="flex items-center gap-xs flex-wrap">
            <h3 className="text-base font-serif font-semibold text-text">{title}</h3>
            <span
              className="inline-flex items-center rounded px-xs py-4xs text-xs font-mono bg-surface-2 border border-border text-text-2"
              title="Execution target environment"
            >
              {target}
            </span>
          </div>
          <p className="text-xs text-text-3 mt-4xs leading-relaxed">{rationale}</p>
        </div>
        <StatusPill tone={toneMap[status]} data-testid="command-step-status">
          {labelMap[status]}
        </StatusPill>
      </div>

      <div className="relative flex items-center justify-between rounded bg-surface-2 border border-border p-xs overflow-hidden">
        <code className="text-xs font-mono text-text px-xs overflow-x-auto whitespace-pre select-all">
          {command}
        </code>
        <button
          type="button"
          onClick={handleCopy}
          aria-label={`Copy command: ${command}`}
          className="ml-xs shrink-0 inline-flex items-center rounded px-sm py-4xs text-xs font-ui font-medium border border-border bg-surface hover:bg-surface-3 text-text transition-colors"
        >
          {copied ? 'Copied!' : 'Copy'}
        </button>
      </div>

      {status === 'failed' && (
        <div
          role="alert"
          className="rounded border border-error/40 bg-error/10 p-sm text-xs text-error flex flex-col gap-xs"
        >
          <div className="font-medium">Command verification did not succeed.</div>
          {errorMessage && <div>{errorMessage}</div>}
          <div className="text-text-2">
            Make sure the command ran completely in your terminal, then try verifying again.
          </div>
          {fallbackText && (
            <div className="mt-xs pt-xs border-t border-border/50 text-text-2">
              <span className="font-medium text-text">Fallback: </span>
              {fallbackText}
            </div>
          )}
        </div>
      )}

      <div className="flex items-center justify-between gap-xs pt-xs flex-wrap">
        <div className="text-xs text-text-3">
          {status === 'verified'
            ? 'Requirement satisfied.'
            : 'Execute the command in your terminal, then verify.'}
        </div>
        <div className="flex items-center gap-xs">
          {status === 'failed' && onFallback && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={onFallback}
              className="text-xs"
            >
              {fallbackLabel}
            </Button>
          )}
          <Button
            type="button"
            variant={status === 'verified' ? 'secondary' : 'primary'}
            size="sm"
            onClick={() => void onVerify()}
            disabled={isBusy || status === 'verified'}
            className="text-xs"
          >
            {isBusy ? 'Verifying…' : status === 'verified' ? 'Verified' : 'Verify Now'}
          </Button>
        </div>
      </div>
    </div>
  )
}
