import type { HTMLAttributes } from 'react'
import { VisuallyHidden } from '../VisuallyHidden/VisuallyHidden'
import { cx } from '../../lib/cx'

export interface SpinnerProps extends HTMLAttributes<HTMLDivElement> {
  label?: string
}

/** role="status" announces the loading state itself — the thing a sighted user sees appear. */
export function Spinner({ label = 'Loading', className, ...rest }: SpinnerProps) {
  return (
    <div role="status" className={cx('inline-flex items-center', className)} {...rest}>
      <span
        aria-hidden="true"
        className={cx(
          'size-2xl rounded-4xl border-2 border-border border-t-accent',
          'animate-spin motion-reduce:animate-none',
        )}
      />
      <VisuallyHidden>{label}</VisuallyHidden>
    </div>
  )
}
