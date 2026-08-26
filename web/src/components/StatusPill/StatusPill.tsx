import type { HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'

export type StatusTone = 'neutral' | 'success' | 'warning' | 'error' | 'accent'

export interface StatusPillProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: StatusTone
}

const TONE: Record<StatusTone, string> = {
  neutral: 'bg-surface-3 text-text-2',
  success: 'bg-success/15 text-success',
  warning: 'bg-warm/15 text-warm',
  error: 'bg-error/15 text-error',
  accent: 'bg-accent-soft text-accent',
}

/** Text always carries the meaning — tone color is a reinforcement, never the only signal. */
export function StatusPill({ tone = 'neutral', className, children, ...rest }: StatusPillProps) {
  return (
    <span
      className={cx(
        'inline-flex items-center gap-4xs rounded-4xl px-sm py-4xs text-xs font-ui',
        TONE[tone],
        className,
      )}
      {...rest}
    >
      {children}
    </span>
  )
}
