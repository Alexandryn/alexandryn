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
import { ChevronLeftIcon, FolderIcon } from '../../components/Icon'
import { AddToCollectionModal } from './AddToCollectionModal'

type TabType = 'about' | 'editions' | 'sources'

/**
 * Work detail screen at /book/:id.
 * Faithfully styled to match the Alexandryn Electron interactive prototype (atBook).
 */
export function WorkDetail() {
  const navigate = useNavigate()
  const params = useParams<{ id?: string; '*'?: string }>()
  const id =
    params.id || (params['*'] ? params['*'].replace(/^book\//, '').split('/')[0] : '') || ''
  const { data: work, error, isPending, refetch } = useWork(id)
  const [activeTab, setActiveTab] = useState<TabType>('about')
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
            mascotMood="searching"
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

  const firstEpubEdition = work.ownedEditions?.find((ed) =>
    ed.formats.some((f) => f.toLowerCase() === 'epub'),
  )

  const firstYear =
    work.ownedEditions?.find((e) => e.publicationYear)?.publicationYear ?? null

  const subjects = work.subjects || []
  const firstSubject = subjects[0] || 'Literature'
  const ownedCount = work.ownedEditions?.length || 0

  const TABS: { id: TabType; label: string; count?: number }[] = [
    { id: 'about', label: 'About' },
    { id: 'editions', label: 'Editions', count: ownedCount },
    { id: 'sources', label: 'Sources', count: ownedCount },
  ]

  const handleTabKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>, currentTab: TabType) => {
    const currentIndex = TABS.findIndex((t) => t.id === currentTab)
    let nextIndex = currentIndex

    if (e.key === 'ArrowRight') {
      e.preventDefault()
      nextIndex = (currentIndex + 1) % TABS.length
    } else if (e.key === 'ArrowLeft') {
      e.preventDefault()
      nextIndex = (currentIndex - 1 + TABS.length) % TABS.length
    } else if (e.key === 'Home') {
      e.preventDefault()
      nextIndex = 0
    } else if (e.key === 'End') {
      e.preventDefault()
      nextIndex = TABS.length - 1
    }

    if (nextIndex !== currentIndex) {
      const nextTab = TABS[nextIndex].id
      setActiveTab(nextTab)
      const tabEl = document.getElementById(`tab-${nextTab}`)
      tabEl?.focus()
    }
  }

  return (
    <div className="p-xl sm:p-2xl md:p-3xl content-container-wide">
      {/* Back navigation */}
      <div className="mb-lg">
        <Link
          to="/library"
          className={cx(
            'inline-flex items-center gap-xs text-xs text-text-2 hover:text-text rounded-2xs transition-colors cursor-pointer',
            FOCUS_RING,
          )}
        >
          <ChevronLeftIcon className="size-3.5 shrink-0" aria-hidden="true" />
          <span>Back to Library</span>
        </Link>
      </div>

      {/* Two-column prototype grid */}
      <div className="book-detail-grid">
        {/* Sticky Left Column: Cover & Primary Actions */}
        <div className="book-detail-sidebar flex flex-col gap-md">
          {/* Tactical Large Book Cover */}
          <div className="book-detail-cover bg-surface-3">
            <div className="book-cover-pattern" />
            <div className="absolute left-0 top-0 bottom-0 w-2.5 bg-black/20 pointer-events-none z-10" />
            <GeneratedCover
              identifier={work.id}
              title={work.title}
              author={coverAuthor}
            />
          </div>

          {/* Action buttons */}
          <div className="flex flex-col gap-xs mt-2xs">
            {firstEpubEdition ? (
              <Link
                to={`/read/${work.id}/${firstEpubEdition.id}`}
                data-testid="read-edition-btn"
                className={cx(
                  'h-10 sm:h-9 min-h-10 sm:min-h-9 rounded-2xs bg-accent text-accent-text font-medium text-2xl flex items-center justify-center transition-colors hover:bg-accent/90 cursor-pointer shadow-sm',
                  FOCUS_RING,
                )}
              >
                Read
              </Link>
            ) : (
              <button
                type="button"
                disabled
                className="h-10 sm:h-9 min-h-10 sm:min-h-9 rounded-2xs bg-surface-3 text-text-3 font-medium text-2xl flex items-center justify-center cursor-not-allowed border border-border"
              >
                Read
              </button>
            )}

            <div className="flex gap-xs">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setIsManageCollectionsOpen(true)}
                className="flex-1"
              >
                + Add to collection
              </Button>
              <button
                type="button"
                aria-label="More book options"
                className={cx(
                  'w-8 h-8 rounded-2xs border border-border bg-surface text-text-2 hover:text-text hover:bg-surface-2 transition-colors flex items-center justify-center cursor-pointer shadow-xs text-sm',
                  FOCUS_RING,
                )}
              >
                ···
              </button>
            </div>

            {/* In Library status card */}
            {ownedCount > 0 ? (
              <div className="flex items-center gap-xs p-sm rounded-2xs bg-surface-2 border border-border">
                <span className="size-1.5 rounded-full bg-success flex-none" />
                <div className="text-xs text-text-2 leading-tight">
                  In your library · <span className="text-text font-medium uppercase font-mono text-3xs">EPUB</span> from connected sources
                </div>
              </div>
            ) : (
              <div className="rounded-xs border border-dashed border-border p-md text-center">
                <p className="text-sm font-medium text-text">Not yet in your library</p>
                <p className="text-xs text-text-2 mt-4xs">
                  This work is on your wanted list or in a collection without an owned edition.
                </p>
              </div>
            )}
          </div>

          {/* Open Library Metadata card */}
          <div className="pt-sm border-t border-border mt-xs">
            <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
              METADATA · OPEN LIBRARY
            </div>
            <div className="font-mono text-xs text-text-2 mt-3xs truncate">
              {work.id}
            </div>
            <div className="text-xs text-text-3 mt-4xs">
              Work record synced recently.
            </div>
          </div>
        </div>

        {/* Right Column: Metadata, Stats Strip & Tabs */}
        <div className="min-w-0 max-w-3xl">
          {/* Breadcrumb Year · Subject */}
          <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
            {firstYear ? `${firstYear} · ` : ''}{firstSubject}
          </div>

          {/* Monumental 52px Newsreader Title */}
          <h1 className="book-detail-title text-text mt-sm mb-xs">
            {work.title}
          </h1>

          {/* Subtitle */}
          {work.subtitle ? (
            <p className="text-lg text-text-2 mb-xs">{work.subtitle}</p>
          ) : null}

          {/* Author */}
          <div className="text-base text-text-2 mt-xs">
            {work.authors && work.authors.length > 0 ? (
              <span>by <strong className="text-text font-medium">{work.authors.join(', ')}</strong></span>
            ) : (
              'Unknown Author'
            )}
          </div>

          {/* Horizontal Stat Strip */}
          <div className="stat-strip mt-lg mb-lg">
            <div>
              <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
                FIRST PUBLISHED
              </div>
              <div className="text-2xl text-text font-medium mt-4xs">
                {firstYear || '—'}
              </div>
            </div>
            <div>
              <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
                EDITIONS
              </div>
              <div className="text-2xl text-text font-medium mt-4xs">
                {ownedCount}
              </div>
            </div>
            <div>
              <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
                PAGES
              </div>
              {/* NOTE(backend-gap): Page count is not currently provided by the backend API schema; displaying placeholder */}
              <div className="text-2xl text-text font-medium mt-4xs">
                —
              </div>
            </div>
            <div>
              <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
                LANGUAGE
              </div>
              <div className="text-2xl text-text font-medium mt-4xs uppercase">
                {work.originalLanguage || 'en'}
              </div>
            </div>
            <div>
              <div className="font-mono text-3xs tracking-wider text-text-3 uppercase">
                SOURCES
              </div>
              <div className="text-2xl text-success font-medium mt-4xs">
                {ownedCount > 0 ? `${ownedCount} available` : 'None connected'}
              </div>
            </div>
          </div>

          {/* Prototype Tabs: About, Editions, Sources */}
          <div
            role="tablist"
            aria-label="Book detail sections"
            className="flex gap-lg border-b border-border mb-lg"
          >
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id
              return (
                <button
                  key={tab.id}
                  id={`tab-${tab.id}`}
                  role="tab"
                  aria-selected={isActive}
                  aria-controls={`tabpanel-${tab.id}`}
                  tabIndex={isActive ? 0 : -1}
                  type="button"
                  onClick={() => setActiveTab(tab.id)}
                  onKeyDown={(e) => handleTabKeyDown(e, tab.id)}
                  className={cx(
                    'relative pb-sm text-xl font-medium transition-colors cursor-pointer flex items-center gap-2xs outline-none',
                    isActive ? 'text-text' : 'text-text-2 hover:text-text',
                    FOCUS_RING,
                  )}
                >
                  <span>{tab.label}</span>
                  {tab.count !== undefined && (
                    <span className="font-mono text-3xs text-text-3">{tab.count}</span>
                  )}
                  {isActive && <div className="tab-active-indicator" />}
                </button>
              )
            })}
          </div>

          {/* Tab 1: About */}
          {activeTab === 'about' && (
            <div
              role="tabpanel"
              id="tabpanel-about"
              aria-labelledby="tab-about"
              tabIndex={0}
              className="flex flex-col gap-xl outline-none"
            >
              {/* Literary Description */}
              {work.subtitle ? (
                <div className="book-reading-desc text-text">
                  {work.title}: {work.subtitle}. An enduring literary classic exploring intricate themes and timeless narratives.
                </div>
              ) : (
                <div className="book-reading-desc text-text">
                  {work.title}, written by {work.authors?.join(', ') || 'Unknown Author'}. Available in your personal digital library.
                </div>
              )}

              {/* Subjects */}
              {subjects.length > 0 && (
                <div>
                  <div className="font-mono text-3xs tracking-wider text-text-3 uppercase mb-xs">
                    SUBJECTS
                  </div>
                  <div className="flex flex-wrap gap-2xs">
                    {subjects.map((sub) => (
                      <span
                        key={sub}
                        className="rounded-4xl border border-border bg-surface px-md py-4xs text-xs text-text-2"
                      >
                        {sub}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Collections Membership */}
              <div>
                <div className="font-mono text-3xs tracking-wider text-text-3 uppercase mb-xs">
                  COLLECTIONS
                </div>
                {work.collections && work.collections.length > 0 ? (
                  <div className="flex flex-wrap gap-xs">
                    {work.collections.map((c) => (
                      <Link
                        key={c.id}
                        to={`/collections/${c.id}`}
                        className={cx(
                          'inline-flex items-center gap-xs rounded-2xs border border-border bg-surface px-md py-xs text-sm font-medium text-text hover:bg-surface-2 transition-colors',
                          FOCUS_RING,
                        )}
                      >
                        <FolderIcon className="size-3.5 text-text-3" aria-hidden="true" />
                        <span>{c.name}</span>
                        {c.addedAt && (
                          <span className="text-3xs text-text-3 font-mono">
                            · added {new Date(c.addedAt).toLocaleDateString()}
                          </span>
                        )}
                      </Link>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-text-3">Not currently in any collection.</p>
                )}
              </div>

              {/* 2-Column Metadata Key-Value Table */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-xl max-w-xl border-t border-border pt-md">
                <div className="flex justify-between py-xs border-b border-border/60">
                  <span className="font-mono text-3xs tracking-wide text-text-3 uppercase">WORK IDENTIFIER</span>
                  <span className="font-mono text-xs text-text truncate max-w-[12rem]">{work.id}</span>
                </div>
                <div className="flex justify-between py-xs border-b border-border/60">
                  <span className="font-mono text-3xs tracking-wide text-text-3 uppercase">ORIGINAL LANGUAGE</span>
                  <span className="text-xs text-text uppercase font-mono">{work.originalLanguage || 'en'}</span>
                </div>
                <div className="flex justify-between py-xs border-b border-border/60">
                  <span className="font-mono text-3xs tracking-wide text-text-3 uppercase">TOTAL EDITIONS</span>
                  <span className="text-xs text-text font-medium">{ownedCount}</span>
                </div>
                <div className="flex justify-between py-xs border-b border-border/60">
                  <span className="font-mono text-3xs tracking-wide text-text-3 uppercase">STATUS</span>
                  <span className="text-xs text-success font-medium">{ownedCount > 0 ? 'In Library' : 'Wanted'}</span>
                </div>
              </div>

              {/* Available from your sources section */}
              {ownedCount > 0 && (
                <div>
                  <div className="flex items-center gap-xs mb-sm">
                    <span className="font-mono text-3xs tracking-wider text-text-3 uppercase">
                      AVAILABLE FROM YOUR SOURCES
                    </span>
                    <div className="flex-1 h-px bg-border" />
                  </div>

                  <div className="flex flex-col gap-xs">
                    {work.ownedEditions.map((ed) => {
                      const hasEpub = ed.formats.some((f) => f.toLowerCase() === 'epub')
                      return (
                        <div
                          key={ed.id}
                          className="flex flex-col sm:flex-row sm:items-center justify-between gap-md p-md rounded-xs border border-border bg-surface shadow-xs"
                        >
                          <div className="flex items-center gap-md min-w-0">
                            <div className="source-avatar-sm bg-surface-3 flex items-center justify-center font-mono text-3xs text-text-2 shrink-0">
                              OPDS
                            </div>
                            <div className="min-w-0">
                              <div className="text-sm font-medium text-text truncate">
                                {ed.publisher || 'Unknown Publisher'}
                              </div>
                              <div className="flex items-center gap-xs mt-4xs">
                                <span className="size-1.5 rounded-full bg-success flex-none" />
                                <span className="font-mono text-3xs text-text-3">
                                  {ed.publicationYear ? `${ed.publicationYear} · ` : ''}
                                  {ed.language}
                                  {ed.isbn ? ` · ISBN ${ed.isbn}` : ''}
                                </span>
                              </div>
                            </div>
                          </div>

                          <div className="flex items-center gap-sm">
                            <div className="flex gap-4xs">
                              {ed.formats.map((fmt) => (
                                <FormatBadge key={fmt} format={fmt} />
                              ))}
                            </div>
                            {hasEpub && (
                              <Link
                                to={`/read/${work.id}/${ed.id}`}
                                className={cx(
                                  'h-7 px-md rounded-2xs bg-surface border border-border text-xs font-medium text-text hover:bg-surface-2 transition-colors flex items-center justify-center cursor-pointer',
                                  FOCUS_RING,
                                )}
                              >
                                Read
                              </Link>
                            )}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                  <p className="text-xs text-text-3 mt-xs">
                    Alexandryn indexes what your connected sources make available. Metadata comes from Open Library; files come from your sources.
                  </p>
                </div>
              )}
            </div>
          )}

          {/* Tab 2: Editions */}
          {activeTab === 'editions' && (
            <div
              role="tabpanel"
              id="tabpanel-editions"
              aria-labelledby="tab-editions"
              tabIndex={0}
              className="flex flex-col gap-sm outline-none"
            >
              <div className="flex flex-wrap items-center justify-between gap-sm p-sm rounded-xs bg-surface-2 border border-border">
                <div className="flex items-center gap-xs text-xs font-mono text-text-3">
                  <span className="uppercase">WORK</span>
                  <span>→</span>
                  <span className="font-serif text-text text-sm">{work.title}</span>
                  <span>→</span>
                  <span className="uppercase">{ownedCount} EDITIONS</span>
                </div>
              </div>

              {work.ownedEditions && work.ownedEditions.length > 0 ? (
                <div className="flex flex-col gap-xs">
                  {work.ownedEditions.map((edition) => {
                    const hasEpub = edition.formats.some(
                      (f) => f.toLowerCase() === 'epub',
                    )
                    return (
                      <div
                        key={edition.id}
                        className="flex flex-col sm:flex-row sm:items-center justify-between gap-md p-md rounded-xs border border-border bg-surface"
                      >
                        <div className="flex items-center gap-md min-w-0">
                          <div className="w-8 aspect-[2/3] rounded-4xs bg-surface-3 border border-border flex-none" />
                          <div className="min-w-0">
                            <div className="text-sm font-medium text-text">
                              {edition.publisher || 'Unknown Publisher'}
                            </div>
                            <div className="font-mono text-3xs text-text-3 mt-4xs">
                              {edition.publicationYear ? `${edition.publicationYear} · ` : ''}
                              {edition.language}
                              {edition.isbn ? ` · ${edition.isbn}` : ''}
                            </div>
                          </div>
                        </div>

                        <div className="flex items-center gap-md">
                          <div className="flex gap-4xs">
                            {edition.formats.map((fmt) => (
                              <FormatBadge key={fmt} format={fmt} />
                            ))}
                          </div>
                          {hasEpub && (
                            <Link
                              to={`/read/${work.id}/${edition.id}`}
                              data-testid="read-edition-btn"
                              className={cx(
                                'h-7 px-md rounded-2xs bg-accent text-accent-text text-xs font-medium hover:bg-accent/90 transition-colors flex items-center justify-center cursor-pointer',
                                FOCUS_RING,
                              )}
                            >
                              Read
                            </Link>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              ) : (
                <div className="p-xl text-center text-text-3 text-sm">
                  No editions available.
                </div>
              )}
            </div>
          )}

          {/* Tab 3: Sources */}
          {activeTab === 'sources' && (
            <div
              role="tabpanel"
              id="tabpanel-sources"
              aria-labelledby="tab-sources"
              tabIndex={0}
              className="flex flex-col gap-md outline-none"
            >
              {ownedCount > 0 ? (
                <div className="flex flex-col gap-sm">
                  <div className="p-lg rounded-md border border-border bg-surface shadow-xs">
                    <div className="flex items-center gap-md">
                      <div className="source-avatar-sm bg-surface-3 flex items-center justify-center font-mono text-2xs text-text-2">
                        OPDS
                      </div>
                      <div className="flex-1">
                        <div className="text-sm font-medium text-text">Connected Library Sources</div>
                        <div className="font-mono text-3xs text-text-3 mt-4xs">Indexed via local and network sources</div>
                      </div>
                      <div className="flex items-center gap-xs px-md py-4xs rounded-4xl bg-surface-2 border border-border">
                        <span className="size-1.5 rounded-full bg-success flex-none" />
                        <span className="font-mono text-3xs text-text-2">Synchronized</span>
                      </div>
                    </div>
                  </div>
                </div>
              ) : (
                <div className="p-xl rounded-md border border-dashed border-border text-center text-text-3 text-sm">
                  Not available from any connected sources.
                </div>
              )}

              <div className="flex items-center gap-sm p-md rounded-md border border-dashed border-border text-text-3 text-xs">
                <span className="size-1.5 rounded-full bg-text-3 flex-none" />
                <span className="flex-1">Manage connected sources and storage locations.</span>
                <Link to="/sources" className="text-accent hover:underline cursor-pointer">
                  Manage sources →
                </Link>
              </div>
            </div>
          )}
        </div>
      </div>

      <AddToCollectionModal
        open={isManageCollectionsOpen}
        onOpenChange={setIsManageCollectionsOpen}
        work={work}
      />
    </div>
  )
}
