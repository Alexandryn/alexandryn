import type { HTMLAttributes } from 'react'
import { VisuallyHidden } from '../VisuallyHidden/VisuallyHidden'
import { cx } from '../../lib/cx'
import { useAnnouncedText } from '../../lib/useAnnouncedText'

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
  label?: string
}

/** role="status" announces the loading placeholder once, the same transition a sighted user sees. */
export function Skeleton({ label = 'Loading content', className, ...rest }: SkeletonProps) {
  const announcedRef = useAnnouncedText<HTMLSpanElement>(label)
  return (
    <div role="status" className={cx('block', className)} {...rest}>
      <div
        aria-hidden="true"
        className={cx(
          'size-full rounded-sm bg-surface-3 animate-pulse motion-reduce:animate-none',
          // Outline the shape under prefers-contrast — the surface-3 fill
          // alone is near-invisible on the page (frontend-accessibility.md FR-5).
          'contrast-more:border contrast-more:border-text-3',
        )}
      />
      <VisuallyHidden ref={announcedRef} />
    </div>
  )
}
