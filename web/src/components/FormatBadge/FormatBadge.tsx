import type { HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'

export interface FormatBadgeProps extends HTMLAttributes<HTMLSpanElement> {
  /** File format label, e.g. "EPUB", "PDF" — the domain layer owns the real vocabulary. */
  format: string
}

export function FormatBadge({ format, className, ...rest }: FormatBadgeProps) {
  return (
    <span
      className={cx(
        'inline-flex items-center rounded-3xs border border-border-2 bg-surface-2',
        'px-2xs py-4xs text-3xs font-mono tracking-6 text-text-2 uppercase',
        className,
      )}
      {...rest}
    >
      {format}
    </span>
  )
}
