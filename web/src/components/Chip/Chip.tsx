import type { HTMLAttributes, ReactNode } from 'react'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface ChipProps extends Omit<HTMLAttributes<HTMLSpanElement>, 'children'> {
  children: ReactNode
  onRemove?: () => void
  /** Accessible name for the remove control. Recommended when children isn't plain text — falls back to a generic "Remove" so the control is never silently unlabelled. */
  removeLabel?: string
  disabled?: boolean
}

export function Chip({ children, onRemove, removeLabel, disabled, className, ...rest }: ChipProps) {
  // Falls back to a generic label rather than undefined when children isn't a
  // plain string — an icon-only remove button must never ship with no
  // accessible name at all, even if the caller forgets removeLabel.
  const fallbackLabel = typeof children === 'string' ? `Remove ${children}` : 'Remove'
  const label = removeLabel ?? fallbackLabel

  return (
    <span
      className={cx(
        'inline-flex items-center gap-4xs rounded-4xl bg-surface-3 text-text px-sm py-4xs text-xs',
        disabled && 'opacity-50',
        className,
      )}
      {...rest}
    >
      <span>{children}</span>
      {onRemove && (
        <button
          type="button"
          onClick={onRemove}
          disabled={disabled}
          aria-label={label}
          className={cx(
            'rounded-4xl hover:bg-surface',
            FOCUS_RING,
            'disabled:opacity-50 disabled:cursor-not-allowed',
          )}
        >
          <span aria-hidden="true">×</span>
        </button>
      )}
    </span>
  )
}
