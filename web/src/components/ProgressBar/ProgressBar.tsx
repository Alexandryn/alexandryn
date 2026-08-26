import { useId, type HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'

export interface ProgressBarProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  label: string
  /** Omit for an indeterminate (in-progress, unknown-duration) bar. */
  value?: number
  max?: number
}

/**
 * Wraps the native <progress> element — its role/aria-valuenow/aria-valuemax
 * are computed by the browser from value/max, no manual ARIA needed (FR-1).
 */
export function ProgressBar({ label, value, max = 100, className, ...rest }: ProgressBarProps) {
  const labelId = useId()
  const indeterminate = value === undefined

  return (
    <div className={cx('flex flex-col gap-4xs', className)} {...rest}>
      <span id={labelId} className="text-xs text-text-2 font-ui">
        {label}
      </span>
      <progress
        aria-labelledby={labelId}
        value={indeterminate ? undefined : value}
        max={max}
        className={cx(
          'w-full h-2xs rounded-4xl overflow-hidden bg-surface-3 appearance-none',
          '[&::-webkit-progress-bar]:bg-surface-3 [&::-webkit-progress-bar]:rounded-4xl',
          '[&::-webkit-progress-value]:bg-accent [&::-webkit-progress-value]:rounded-4xl',
          '[&::-moz-progress-bar]:bg-accent [&::-moz-progress-bar]:rounded-4xl',
          indeterminate && 'animate-pulse motion-reduce:animate-none',
        )}
      />
    </div>
  )
}
