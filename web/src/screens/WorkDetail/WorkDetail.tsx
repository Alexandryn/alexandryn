import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { FormatBadge } from '../../components/FormatBadge/FormatBadge'
import { GeneratedCover } from '../../components/GeneratedCover/GeneratedCover'
import { Spinner } from '../../components/Spinner/Spinner'
import { ApiError } from '../../data/http'
import { useWork } from '../../data/library'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { AddToCollectionModal } from './AddToCollectionModal'

/**
 * Work detail screen at /book/:id (frontend-library-screens.md FR-5).
 * Displays work metadata, owned editions with format badges and 'Read' action
 * for EPUB editions, and collection memberships.
 */
export function WorkDetail() {
  const navigate = useNavigate()
  const params = useParams<{ id?: string; '*'?: string }>()
  const id =
    params.id || (params['*'] ? params['*'].replace(/^book\//, '').split('/')[0] : '') || ''
  const { data: work, error, isPending, refetch } = useWork(id)
  const [isManageCollectionsOpen, setIsManageCollectionsOpen] = useState(false)



  if (isPending) {
    return <Spinner label="Loading book details" className="m-3xl" />
  }

  if (error) {
    const is404 = error instanceof ApiError && error.status === 404
    if (is404) {
      return (
        <div className="p-3xl">
          <EmptyState
            title="This book isn't in your library."
            description="We couldn't find a book matching that identifier."
            action={{
              label: 'Back to Library',
              onClick: () => navigate('/library'),
            }}
          />
        </div>
      )
    }

    return (
      <div className="p-3xl">
        <ErrorState
          title="Couldn't load book details"
          description={error instanceof ApiError ? error.message : undefined}
          code={error instanceof ApiError ? error.code : undefined}
          correlationId={error instanceof ApiError ? error.correlationId : undefined}
          onRetry={() => void refetch()}
        />
      </div>
    )
  }

  if (!work) return null

  const coverAuthor =
    work.authors && work.authors.length > 1
      ? `${work.authors[0]} et al.`
      : work.authors?.[0]

  return (
    <div className="flex flex-col gap-2xl p-3xl max-w-4xl">
      {/* Back navigation */}
      <div>
        <Link
          to="/library"
          className={cx(
            'inline-flex items-center gap-2xs text-sm font-medium text-text-2 hover:text-text rounded-2xs',
            FOCUS_RING,
          )}
        >
          ← Back to Library
        </Link>
      </div>

      {/* Main Metadata Section */}
      <div className="flex flex-col sm:flex-row gap-xl items-start">
        <div className="w-40 sm:w-48 shrink-0 aspect-[2/3] overflow-hidden rounded-xs bg-surface-3 shadow-md">
          <GeneratedCover
            identifier={work.id}
            title={work.title}
            author={coverAuthor}
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

          <p className="text-base text-text-2">
            {work.authors && work.authors.length > 0 ? (
              <span>by <strong className="text-text font-medium">{work.authors.join(', ')}</strong></span>
            ) : (
              'Unknown Author'
            )}
          </p>

          {work.originalLanguage ? (
            <p className="text-xs text-text-3 uppercase tracking-wider font-mono">
              Original Language: {work.originalLanguage}
            </p>
          ) : null}

          {work.subjects && work.subjects.length > 0 ? (
            <div className="flex flex-wrap gap-2xs mt-xs">
              {work.subjects.map((sub) => (
                <span
                  key={sub}
                  className="rounded-4xl bg-surface-3 px-sm py-4xs text-xs text-text-2"
                >
                  {sub}
                </span>
              ))}
            </div>
          ) : null}
        </div>
      </div>

      {/* Collections Section */}
      <div className="flex flex-col gap-md border-t border-border pt-lg">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-medium text-text">Collections</h2>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setIsManageCollectionsOpen(true)}
          >
            + Add to collection
          </Button>
        </div>

        {work.collections && work.collections.length > 0 ? (
          <div className="flex flex-wrap gap-sm">
            {work.collections.map((c) => (
              <Link
                key={c.id}
                to={`/collections/${c.id}`}
                className={cx(
                  'inline-flex items-center gap-xs rounded-sm border border-border bg-surface px-md py-xs text-sm font-medium text-text transition-colors hover:border-text-3 hover:bg-surface-2',
                  FOCUS_RING,
                )}
              >
                <span>📁 {c.name}</span>
                {c.addedAt ? (
                  <span className="text-xs text-text-3">
                    · added {new Date(c.addedAt).toLocaleDateString()}
                  </span>
                ) : null}
              </Link>
            ))}
          </div>
        ) : (
          <p className="text-sm text-text-2">Not currently in any collection.</p>
        )}
      </div>

      <AddToCollectionModal
        open={isManageCollectionsOpen}
        onOpenChange={setIsManageCollectionsOpen}
        work={work}
      />


      {/* Owned Editions Section */}
      <div className="flex flex-col gap-md border-t border-border pt-lg">
        <h2 className="text-xl font-medium text-text">Owned Editions</h2>
        {work.ownedEditions && work.ownedEditions.length > 0 ? (
          <div className="flex flex-col gap-sm">
            {work.ownedEditions.map((edition) => {
              const hasEpub = edition.formats.some(
                (f) => f.toLowerCase() === 'epub',
              )

              return (
                <div
                  key={edition.id}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-md rounded-xs border border-border bg-surface p-md"
                >
                  <div className="flex flex-col gap-4xs">
                    <div className="flex items-center gap-sm">
                      <span className="font-medium text-text text-sm">
                        {edition.publisher || 'Unknown Publisher'}
                      </span>
                      {edition.publicationYear ? (
                        <span className="text-text-2 text-xs">
                          ({edition.publicationYear})
                        </span>
                      ) : null}
                      <span className="text-text-3 text-xs uppercase font-mono">
                        {edition.language}
                      </span>
                    </div>

                    {edition.isbn ? (
                      <span className="text-xs font-mono text-text-3">
                        ISBN: {edition.isbn}
                      </span>
                    ) : null}

                    {edition.addedAt ? (
                      <span className="text-xs text-text-2">
                        Added on {new Date(edition.addedAt).toLocaleDateString()}
                      </span>
                    ) : null}

                    <div className="flex items-center gap-2xs mt-4xs">
                      {edition.formats.map((fmt) => (
                        <FormatBadge key={fmt} format={fmt} />
                      ))}
                    </div>
                  </div>

                  {/* Read button for EPUB formats (phase 11 amendment) */}
                  {hasEpub && (
                    <Link
                      to={`/read/${work.id}/${edition.id}`}
                      data-testid="read-edition-btn"
                      className={cx(
                        'inline-flex items-center justify-center gap-xs rounded-md font-ui transition-colors px-sm py-4xs text-xs bg-accent text-accent-text hover:bg-accent/90 self-start sm:self-center shrink-0',
                        FOCUS_RING,
                      )}
                    >
                      Read
                    </Link>
                  )}
                </div>
              )
            })}
          </div>
        ) : (
          <div className="rounded-xs border border-dashed border-border p-lg text-center">
            <p className="text-sm font-medium text-text">Not yet in your library</p>
            <p className="text-xs text-text-2 mt-4xs">
              This work is on your wanted list or in a collection without an owned edition.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
