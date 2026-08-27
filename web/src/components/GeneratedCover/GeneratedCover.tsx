import type { HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import { TextureLayer } from './TextureLayer'
import { SpineLayer } from './SpineLayer'
import { TitleLayer } from './TitleLayer'
import { AuthorLayer } from './AuthorLayer'
import { getCachedSeed } from './seedCache'

export interface GeneratedCoverProps extends Omit<
  HTMLAttributes<HTMLDivElement>,
  'title' | 'aria-hidden'
> {
  /** Work.ID or Edition.ID — the only prop always available (FR-2's seed source). */
  identifier: string
  title?: string
  /** Already-reduced display string (first author + "et al." when multiple) — never a raw Author[]. */
  author?: string
}

/**
 * Four-layer composition (FR-1) with the three-step degradation ladder
 * (FR-3): title+author present → full composition; title only → texture
 * + spine + re-centered title; neither → texture + spine only, never a
 * blank box. Purely decorative — see GeneratedCoverImage's own a11y
 * wiring (T6) for the accessible-name contract.
 */
export function GeneratedCover({
  identifier,
  title,
  author,
  className,
  ...rest
}: GeneratedCoverProps) {
  const seed = getCachedSeed(identifier)

  return (
    <div
      className={cx('relative size-full aspect-[2/3] overflow-hidden rounded-xs', className)}
      {...rest}
      aria-hidden="true"
    >
      <TextureLayer seed={seed} className="absolute inset-0" />
      <SpineLayer seed={seed} />
      {title ? (
        <div
          className={cx(
            'absolute inset-0 flex flex-col gap-4xs p-sm',
            author ? 'justify-end' : 'justify-center',
          )}
        >
          <TitleLayer title={title} />
          {author && <AuthorLayer author={author} />}
        </div>
      ) : null}
    </div>
  )
}
