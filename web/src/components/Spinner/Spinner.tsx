import type { HTMLAttributes } from 'react'
import { VisuallyHidden } from '../VisuallyHidden/VisuallyHidden'
import { cx } from '../../lib/cx'
import { useAnnouncedText } from '../../lib/useAnnouncedText'

export interface SpinnerProps extends HTMLAttributes<HTMLDivElement> {
  label?: string
}

/** role="status" announces the loading state itself — the thing a sighted user sees appear. */
export function Spinner({ label = 'Loading', className, ...rest }: SpinnerProps) {
  const announcedRef = useAnnouncedText<HTMLSpanElement>(label)
  return (
    <div role="status" className={cx('inline-flex items-center', className)} {...rest}>
      <span
        aria-hidden="true"
        className={cx(
          'size-2xl rounded-4xl border-2 border-border border-t-accent',
          // The non-accent part of the ring is near-invisible on the page
          // surface — darken it under prefers-contrast (frontend-accessibility.md
          // FR-5). text-3 itself is bumped to the text-2 value there (a11y.css).
          'contrast-more:border-text-3',
          'animate-spin motion-reduce:animate-none',
        )}
      />
      <VisuallyHidden ref={announcedRef} />
    </div>
  )
}
