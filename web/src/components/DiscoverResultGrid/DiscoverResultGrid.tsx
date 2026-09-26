import type { HTMLAttributes } from 'react'
import { Link } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import type { NormalisedSearchResult } from '../../data/discover'
import { DiscoverCover } from './DiscoverCover'

export interface DiscoverResultGridProps extends Omit<HTMLAttributes<HTMLDivElement>, 'results'> {
  results: NormalisedSearchResult[]
}

function formatAuthorString(authors?: { name: string }[]): string {
  if (!authors || authors.length === 0) return 'Unknown Author'
  return authors.map((a) => a.name).join(', ')
}

function formatCoverAuthor(authors?: { name: string }[]): string | undefined {
  if (!authors || authors.length === 0 || !authors[0]) return undefined
  if (authors.length === 1) return authors[0].name
  return `${authors[0].name} et al.`
}

/**
 * Renders Open Library search results in a responsive grid layout.
 * Grid only, no view toggle, no virtualization needed (bounded to <= 50 results).
 */
export function DiscoverResultGrid({ results, className, ...rest }: DiscoverResultGridProps) {
  return (
    <div className={cx('flex flex-col gap-md', className)} {...rest}>
      <div className="grid grid-cols-2 gap-md sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {results.map((item, idx) => (
          <div key={item.openLibraryWorkKey}>
            <Link
              to={`/discover/works/${encodeURIComponent(item.openLibraryWorkKey)}`}
              className={cx(
                'group flex flex-col rounded-xs p-xs transition-colors hover:bg-surface-2',
                FOCUS_RING,
              )}
            >
              <DiscoverCover
                coverUrl={item.coverUrl}
                identifier={item.openLibraryWorkKey}
                title={item.title}
                author={formatCoverAuthor(item.authors)}
                priority={idx < 6}
              />

              <div className="mt-xs flex flex-col">
                <span className="font-medium text-sm text-text line-clamp-2 leading-snug">
                  {item.title}
                </span>
                <span className="text-xs text-text-2 line-clamp-1 mt-4xs">
                  {formatAuthorString(item.authors)}
                </span>
                <div className="mt-xs flex items-center gap-xs text-3xs text-text-3">
                  {item.firstPublishYear && <span>{item.firstPublishYear}</span>}
                  {item.firstPublishYear && item.editionCount > 0 && <span>•</span>}
                  {item.editionCount > 0 && (
                    <span>
                      {item.editionCount} {item.editionCount === 1 ? 'edition' : 'editions'}
                    </span>
                  )}
                </div>
              </div>
            </Link>
          </div>
        ))}
      </div>
    </div>
  )
}
