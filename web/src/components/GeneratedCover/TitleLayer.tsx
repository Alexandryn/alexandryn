import type { HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'

export interface TitleLayerProps extends Omit<HTMLAttributes<HTMLParagraphElement>, 'children'> {
  title: string
}

/** Pure — wraps and clamps rather than overflowing. Newsreader token typography. */
export function TitleLayer({ title, className, ...rest }: TitleLayerProps) {
  return (
    <p
      className={cx('font-reading text-lg text-text line-clamp-3 break-words', className)}
      {...rest}
    >
      {title}
    </p>
  )
}
