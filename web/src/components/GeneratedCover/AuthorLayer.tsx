import type { HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'

export interface AuthorLayerProps extends Omit<HTMLAttributes<HTMLParagraphElement>, 'children'> {
  /** Already-reduced display string (first author + "et al." when multiple) — this component doesn't reduce an Author[] itself. */
  author: string
}

/** Pure — renders the given display string as-is, smaller than the title layer. */
export function AuthorLayer({ author, className, ...rest }: AuthorLayerProps) {
  return (
    <p className={cx('font-ui text-xs text-text-2 line-clamp-1 break-words', className)} {...rest}>
      {author}
    </p>
  )
}
