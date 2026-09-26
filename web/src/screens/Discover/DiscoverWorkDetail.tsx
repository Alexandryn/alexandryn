import { Link, useNavigate, useParams } from 'react-router-dom'
import { Chip } from '../../components/Chip/Chip'
import { DiscoverCover } from '../../components/DiscoverResultGrid/DiscoverCover'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Spinner } from '../../components/Spinner/Spinner'
import { useDiscoverWork } from '../../data/discover'
import { ApiError } from '../../data/http'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { ChevronLeftIcon } from '../../components/Icon'

/**
 * Open Library work detail screen at /discover/works/:openLibraryId.
 * Displays normalised work metadata, subjects tag list, real cover image with procedural fallback,
 * and list of editions.
 */
export function DiscoverWorkDetail() {
  const navigate = useNavigate()
  const params = useParams<{ openLibraryId?: string }>()
  const openLibraryId = params.openLibraryId ?? ''

  const { data, error, isPending, refetch } = useDiscoverWork(openLibraryId)

  if (isPending) {
    return <Spinner label="Loading book details" className="m-3xl" />
  }

  if (error) {
    const is404 = error instanceof ApiError && (error.status === 404 || error.code.toLowerCase() === 'not_found' || error.code.toLowerCase() === 'notfound')
    if (is404) {
      return (
        <div className="p-3xl">
          <EmptyState
            title="This book couldn't be found on Open Library"
            description="We couldn't find a book matching that Open Library identifier."
            action={{
              label: 'Back to Discover',
              onClick: () => navigate('/discover'),
            }}
          />
        </div>
      )
    }

    const isUnavailable =
      error instanceof ApiError && (error.status === 503 || error.code.toLowerCase() === 'unavailable')

    return (
      <div className="p-3xl">
        <ErrorState
          title={
            isUnavailable
              ? 'Open Library is unavailable right now, try again shortly'
              : "Couldn't load book details"
          }
          description={error instanceof ApiError ? error.message : undefined}
          code={error instanceof ApiError ? error.code : undefined}
          correlationId={error instanceof ApiError ? error.correlationId : undefined}
          onRetry={() => void refetch()}
        />
      </div>
    )
  }

  if (!data || !data.work) return null

  const { work, editions } = data
  const coverAuthor =
    work.authors && work.authors.length > 1 && work.authors[0]
      ? `${work.authors[0].name} et al.`
      : work.authors?.[0]?.name

  const authorNames = work.authors && work.authors.length > 0
    ? work.authors.map((a) => a.name).join(', ')
    : 'Unknown Author'

  return (
    <div className="flex flex-col gap-2xl p-3xl max-w-4xl">
      {/* Back navigation */}
      <div>
        <Link
          to="/discover"
          className={cx(
            'inline-flex items-center gap-1.5 text-sm font-medium text-text-2 hover:text-text rounded-2xs transition-colors',
            FOCUS_RING,
          )}
        >
          <ChevronLeftIcon className="size-4" aria-hidden="true" />
          <span>Discover</span>
        </Link>
      </div>

      {/* Main Metadata Section */}
      <div className="flex flex-col sm:flex-row gap-xl items-start">
        <div className="w-40 sm:w-48 shrink-0">
          <DiscoverCover
            coverUrl={work.coverUrl}
            identifier={openLibraryId}
            title={work.title}
            author={coverAuthor}
            priority
          />
        </div>

        <div className="flex flex-col gap-sm flex-1 min-w-0">
          <div>
            <h1 className="text-3xl font-medium tracking-1 text-text leading-tight">
              {work.title}
            </h1>
            {work.subtitle ? (
              <p className="text-lg text-text-2 mt-2xs">{work.subtitle}</p>
            ) : null}
          </div>

          <p className="text-sm font-medium text-text-2">
            By {authorNames}
          </p>

          {/* Subjects: omitted when absent */}
          {work.subjects && work.subjects.length > 0 ? (
            <div className="flex flex-wrap gap-xs mt-xs" aria-label="Subjects">
              {work.subjects.map((subject) => (
                <Chip key={subject}>{subject}</Chip>
              ))}
            </div>
          ) : null}

          {/* Description: omitted entirely from layout when absent */}
          {work.description ? (
            <div className="mt-md border-t border-border pt-md">
              <h2 className="text-xs font-mono uppercase tracking-1 text-text-3 mb-xs">
                About this work
              </h2>
              <p className="text-sm text-text-2 whitespace-pre-line leading-relaxed">
                {work.description}
              </p>
            </div>
          ) : null}
        </div>
      </div>

      {/* Editions Section */}
      {editions && editions.length > 0 ? (
        <div className="flex flex-col gap-md border-t border-border pt-xl">
          <h2 className="text-xl font-medium text-text">
            Editions ({editions.length})
          </h2>

          <div className="flex flex-col gap-xs">
            {editions.map((ed) => (
              <div
                key={ed.openLibraryEditionKey}
                className="flex items-center justify-between gap-md rounded-xs border border-border bg-surface p-sm"
              >
                <div className="flex flex-col min-w-0">
                  <span className="font-medium text-sm text-text truncate">
                    {ed.title}
                  </span>
                  <div className="flex flex-wrap items-center gap-xs text-xs text-text-2 mt-4xs">
                    {ed.publisher ? <span>{ed.publisher}</span> : null}
                    {ed.publisher && ed.publishDate ? <span>•</span> : null}
                    {ed.publishDate ? <span>{ed.publishDate}</span> : null}
                    {(ed.publisher || ed.publishDate) && ed.language ? <span>•</span> : null}
                    {ed.language ? (
                      <span className="uppercase text-3xs font-mono rounded-3xs border border-border-2 px-2xs py-4xs">
                        {ed.language}
                      </span>
                    ) : null}
                  </div>
                </div>

                <span className="text-3xs font-mono text-text-3 shrink-0">
                  {ed.openLibraryEditionKey}
                </span>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}
