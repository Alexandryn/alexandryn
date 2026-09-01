import { Button } from '../Button/Button'
import { DiscoverCover } from '../DiscoverResultGrid/DiscoverCover'
import { EmptyState } from '../EmptyState/EmptyState'
import { ErrorState } from '../ErrorState/ErrorState'
import { FormatBadge } from '../FormatBadge/FormatBadge'
import { Spinner } from '../Spinner/Spinner'
import type { SourceCandidate } from '../../data/sources'
import { cx } from '../../lib/cx'
import { formatBytes } from './formatBytes'

export interface SourceCandidateListProps {
  candidates: SourceCandidate[]
  hasNextPage?: boolean
  isFetchingNextPage?: boolean
  fetchNextPage?: () => void
  isLoading?: boolean
  error?: Error | null
  onRetry?: () => void
  emptyMessage?: string
  className?: string
}

/**
 * SourceCandidateList (frontend-source-management.md FR-6):
 * Renders candidate items from a source's browse or search response,
 * displaying cover art, metadata, file format badge and file size.
 */
export function SourceCandidateList({
  candidates,
  hasNextPage = false,
  isFetchingNextPage = false,
  fetchNextPage,
  isLoading = false,
  error = null,
  onRetry,
  emptyMessage = 'No books found in this source.',
  className,
}: SourceCandidateListProps) {
  if (isLoading) {
    return <Spinner label="Loading items" className="m-3xl" />
  }

  if (error) {
    return (
      <ErrorState
        title="Could not load books from source"
        description={error.message}
        onRetry={onRetry}
      />
    )
  }

  if (candidates.length === 0) {
    return <EmptyState title="No items found" description={emptyMessage} />
  }

  return (
    <div className={cx('flex flex-col gap-xl', className)}>
      <div
        className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-lg"
        data-testid="source-candidate-grid"
      >
        {candidates.map((candidate, idx) => {
          const sizeStr = formatBytes(candidate.fileReference.sizeBytes)
          return (
            <div
              key={`${candidate.fileReference.referenceId}-${idx}`}
              className="group flex flex-col gap-xs rounded-md bg-surface p-xs transition-colors hover:bg-surface-2"
            >
              <div className="relative aspect-[2/3] w-full overflow-hidden rounded-xs bg-surface-3">
                <DiscoverCover
                  coverUrl={candidate.coverUrl}
                  identifier={candidate.fileReference.referenceId}
                  title={candidate.title}
                  author={candidate.author ?? undefined}
                />
              </div>

              <div className="flex flex-col gap-4xs px-2xs pt-2xs">
                <h2
                  className="text-sm font-medium text-text leading-tight line-clamp-2"
                  title={candidate.title}
                >
                  {candidate.title}
                </h2>
                <p className="text-xs text-text-2 truncate">
                  {candidate.author || 'Unknown author'}
                </p>
                <div className="mt-4xs flex items-center justify-between gap-xs">
                  <FormatBadge format={candidate.fileReference.format} />
                  {sizeStr && <span className="text-3xs font-mono text-text-3">{sizeStr}</span>}
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {hasNextPage && fetchNextPage && (
        <div className="flex justify-center p-md">
          <Button variant="secondary" onClick={fetchNextPage} disabled={isFetchingNextPage}>
            {isFetchingNextPage ? 'Loading more...' : 'Load more'}
          </Button>
        </div>
      )}
    </div>
  )
}
