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
        className="size-full rounded-sm bg-surface-3 animate-pulse motion-reduce:animate-none"
      />
      <VisuallyHidden ref={announcedRef} />
    </div>
  )
}
