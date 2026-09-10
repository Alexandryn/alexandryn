import { useState } from 'react'
import { cx } from '../../lib/cx'
import { GeneratedCover } from '../GeneratedCover/GeneratedCover'

export interface DiscoverCoverProps {
  coverUrl?: string | null
  identifier: string
  title: string
  author?: string
  className?: string
  priority?: boolean
}

/**
 * Renders an Open Library cover with reserved aspect-ratio dimensions and lazy loading (FR-7).
 * Falls back to GeneratedCover procedural rendering if coverUrl is absent or on network image load error.
 */
export function DiscoverCover({
  coverUrl,
  identifier,
  title,
  author,
  className,
  priority = false,
}: DiscoverCoverProps) {
  const [hasError, setHasError] = useState(false)
  const [prevCoverUrl, setPrevCoverUrl] = useState(coverUrl)

  if (coverUrl !== prevCoverUrl) {
    setPrevCoverUrl(coverUrl)
    setHasError(false)
  }

  return (
    <div
      className={cx(
        'relative aspect-[2/3] w-full overflow-hidden rounded-xs bg-surface-3 shadow-sm group-hover:shadow-md transition-shadow',
        className,
      )}
    >
      {coverUrl && !hasError ? (
        <img
          src={coverUrl}
          alt=""
          loading={priority ? 'eager' : 'lazy'}
          fetchPriority={priority ? 'high' : undefined}
          decoding="async"
          onError={() => setHasError(true)}
          className="size-full object-cover"
        />
      ) : (
        <GeneratedCover identifier={identifier} title={title} author={author} />
      )}
    </div>
  )
}
