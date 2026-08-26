import type { HTMLAttributes, ReactNode } from 'react'
import { cx } from '../../lib/cx'

export interface ChipProps extends Omit<HTMLAttributes<HTMLSpanElement>, 'children'> {
  children: ReactNode
  onRemove?: () => void
  /** Accessible name for the remove control; required when children isn't plain text. */
  removeLabel?: string
  disabled?: boolean
}

export function Chip({ children, onRemove, removeLabel, disabled, className, ...rest }: ChipProps) {
  const fallbackLabel = typeof children === 'string' ? `Remove ${children}` : undefined
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
            'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
            'disabled:opacity-50 disabled:cursor-not-allowed',
          )}
        >
          <span aria-hidden="true">×</span>
        </button>
      )}
    </span>
  )
}
