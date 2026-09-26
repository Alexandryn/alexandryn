import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { GeneratedCover } from '../../components/GeneratedCover/GeneratedCover'
import { Spinner } from '../../components/Spinner/Spinner'
import { ApiError } from '../../data/http'
import { useWork } from '../../data/library'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { FolderIcon } from '../../components/Icon'
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
      const nextTab = TABS[nextIndex]?.id
      if (nextTab) {
        setActiveTab(nextTab)
        const tabEl = document.getElementById(`tab-${nextTab}`)
        tabEl?.focus()
      }
    }
  }

  return (
    <div className="book-detail-container">
      {/* Back navigation */}
      <div>
        <Link
          to="/library"
          className={cx('book-detail-back-link', FOCUS_RING)}
        >
          <span aria-hidden="true">←</span>
          <span>Library</span>
        </Link>
      </div>

      {/* Two-column prototype grid */}
      <div className="book-detail-grid">
        {/* Sticky Left Column: Cover & Primary Actions */}
        <div className="book-detail-sidebar">
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
          <div className="flex flex-col gap-2xs mt-md">
            {firstEpubEdition ? (
              <Link
                to={`/read/${work.id}/${firstEpubEdition.id}`}
                data-testid="read-edition-btn"
                className={cx('book-btn-read', FOCUS_RING)}
              >
                Read
              </Link>
            ) : (
              <button
                type="button"
                disabled
                className="book-btn-read-disabled"
              >
                Read
              </button>
            )}

            <div className="flex gap-2xs">
              <button
                type="button"
                onClick={() => setIsManageCollectionsOpen(true)}
                className={cx('book-btn-collection', FOCUS_RING)}
              >
                Add to collection
              </button>
              <button
                type="button"
                aria-label="More book options"
                className={cx('book-btn-more', FOCUS_RING)}
              >
                ···
              </button>
            </div>

            {/* In Library status card */}
            {ownedCount > 0 ? (
              <div className="book-status-card">
                <span className="size-1.5 rounded-full bg-success flex-none" />
                <div className="book-status-card-text">
                  In your library · <span className="text-text font-medium uppercase font-mono text-3xs">EPUB</span> from connected sources
                </div>
              </div>
            ) : (
              <div className="rounded-xs border border-dashed border-border p-md text-center mt-2xs">
                <p className="text-xs font-medium text-text">Not yet in your library</p>
                <p className="text-3xs text-text-3 mt-4xs">
                  This work is on your wanted list or in a collection without an owned edition.
                </p>
              </div>
            )}
          </div>

          {/* Open Library Metadata card */}
          <div className="book-metadata-box">
            <div className="book-metadata-box-header">
              METADATA · OPEN LIBRARY
            </div>
            <div className="book-metadata-box-id">
              {work.id}
            </div>
            <div className="book-metadata-box-note">
              Work record synced recently.
            </div>
          </div>
        </div>

        {/* Right Column: Metadata, Stats Strip & Tabs */}
        <div className="book-detail-content-col">
          {/* Breadcrumb Year · Subject */}
          <div className="book-detail-category">
            {firstYear ? `${firstYear} · ` : ''}{firstSubject}
          </div>

          {/* Monumental 52px Newsreader Title */}
          <h1 className="book-detail-title text-text">
            {work.title}
          </h1>

          {/* Subtitle */}
          {work.subtitle ? (
            <p className="text-base text-text-2 mt-xs">{work.subtitle}</p>
          ) : null}

          {/* Author */}
          <div className="book-detail-author">
            {work.authors && work.authors.length > 0 ? (
              <strong className="font-normal text-text-2">{work.authors.join(', ')}</strong>
            ) : (
              'Unknown Author'
            )}
          </div>

          {/* Horizontal Stat Strip */}
          <div className="stat-strip">
            <div>
              <div className="stat-strip-label">
                FIRST PUBLISHED
              </div>
              <div className="stat-strip-val">
                {firstYear || '—'}
              </div>
            </div>
            <div>
              <div className="stat-strip-label">
                EDITIONS
              </div>
              <div className="stat-strip-val">
                {ownedCount}
              </div>
            </div>
            <div>
              <div className="stat-strip-label">
                PAGES
              </div>
              {/* NOTE(backend-gap): Page count is not currently provided by the backend API schema; displaying placeholder */}
              <div className="stat-strip-val">
                —
              </div>
            </div>
            <div>
              <div className="stat-strip-label">
                LANGUAGE
              </div>
              <div className="stat-strip-val uppercase font-mono">
                {work.originalLanguage || 'en'}
              </div>
            </div>
            <div>
              <div className="stat-strip-label">
                SOURCES
              </div>
              <div className="stat-strip-val-accent">
                {ownedCount > 0 ? `${ownedCount} available` : 'None connected'}
              </div>
            </div>
          </div>

          {/* Prototype Tabs: About, Editions, Sources */}
          <div
            role="tablist"
            aria-label="Book detail sections"
            className="book-tabs-nav"
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
                  className={cx('book-tab-btn', FOCUS_RING)}
                >
                  <span>{tab.label}</span>
                  {tab.count !== undefined && (
                    <span className="book-tab-count">{tab.count}</span>
                  )}
                  {isActive && <div className="book-tab-indicator" />}
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
              className="pt-lg flex flex-col gap-xl outline-none"
            >
              {/* Literary Description */}
              <div>
                <div className="book-reading-desc-primary">
                  {work.subtitle
                    ? `${work.title}: ${work.subtitle}. An enduring literary classic exploring intricate themes and timeless narratives.`
                    : `${work.title}, written by ${work.authors?.join(', ') || 'Unknown Author'}. Available in your personal digital library.`}
                </div>
                <div className="book-reading-desc-secondary">
                  Alexandryn provides unified cataloging, metadata synchronization with Open Library, and direct access across your connected OPDS and local storage backends.
                </div>
              </div>

              {/* Subjects */}
              {subjects.length > 0 && (
                <div>
                  <div className="book-subjects-heading">
                    SUBJECTS
                  </div>
                  <div className="flex flex-wrap gap-2xs">
                    {subjects.map((sub) => (
                      <span
                        key={sub}
                        className="book-subject-pill"
                      >
                        {sub}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Collections Membership */}
              {work.collections && work.collections.length > 0 && (
                <div>
                  <div className="book-subjects-heading">
                    COLLECTIONS
                  </div>
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
                </div>
              )}

              {/* 2-Column Metadata Key-Value Table matching prototype */}
              <div className="book-meta-grid">
                <div className="book-meta-row">
                  <div className="book-meta-key">WORK IDENTIFIER</div>
                  <div className="book-meta-val font-mono">{work.id}</div>
                </div>
                <div className="book-meta-row">
                  <div className="book-meta-key">ORIGINAL LANGUAGE</div>
                  <div className="book-meta-val uppercase font-mono">{work.originalLanguage || 'en'}</div>
                </div>
                <div className="book-meta-row">
                  <div className="book-meta-key">TOTAL EDITIONS</div>
                  <div className="book-meta-val font-medium">{ownedCount}</div>
                </div>
                <div className="book-meta-row">
                  <div className="book-meta-key">STATUS</div>
                  <div className="book-meta-val text-success font-medium">{ownedCount > 0 ? 'In Library' : 'Wanted'}</div>
                </div>
              </div>

              {/* Available from your sources section */}
              {ownedCount > 0 && (
                <div className="book-sources-section">
                  <div className="book-sources-header">
                    <span className="book-subjects-heading mb-0">
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
                          className="book-source-row"
                        >
                          <div className="book-source-badge">
                            OPDS
                          </div>
                          <div className="flex-1 min-w-0">
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
                          <div className="flex gap-4xs">
                            {ed.formats.map((fmt) => (
                              <span key={fmt} className="book-edition-pill uppercase">
                                {fmt}
                              </span>
                            ))}
                          </div>
                          {hasEpub && (
                            <Link
                              to={`/read/${work.id}/${ed.id}`}
                              className={cx('book-source-action-btn', FOCUS_RING)}
                            >
                              Read
                            </Link>
                          )}
                        </div>
                      )
                    })}
                  </div>
                  <div className="book-sources-disclaimer">
                    Alexandryn indexes what your connected sources make available. Metadata comes from Open Library; files come from your sources.
                  </div>
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
              className="pt-lg flex flex-col gap-sm outline-none"
            >
              <div className="book-editions-filter-bar">
                <span className="font-mono text-3xs tracking-wider text-text-3 uppercase">WORK</span>
                <span className="font-serif text-text text-sm">{work.title}</span>
                <span className="text-text-3">→</span>
                <span className="font-mono text-3xs tracking-wider text-text-3 uppercase">{ownedCount} EDITIONS</span>
                <div className="flex-1" />
                <div className="hidden sm:flex gap-2xs">
                  <span className="book-filter-pill">Language ▾</span>
                  <span className="book-filter-pill">Format ▾</span>
                  <span className="book-filter-pill">Available only</span>
                </div>
              </div>

              {work.ownedEditions && work.ownedEditions.length > 0 ? (
                <div className="flex flex-col gap-2xs">
                  {work.ownedEditions.map((edition) => {
                    const hasEpub = edition.formats.some(
                      (f) => f.toLowerCase() === 'epub',
                    )
                    return (
                      <div
                        key={edition.id}
                        className="book-edition-row"
                      >
                        <div className="relative w-8.5 aspect-[2/3] rounded-4xs bg-surface-3 border border-border flex-none overflow-hidden">
                          <div className="absolute left-0 top-0 bottom-0 w-1 bg-black/20" />
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-text truncate">
                            {edition.publisher || 'Unknown Publisher'}
                          </div>
                          <div className="font-mono text-3xs text-text-3 mt-4xs">
                            {edition.publicationYear ? `${edition.publicationYear} · ` : ''}
                            {edition.language}
                            {edition.isbn ? ` · ${edition.isbn}` : ''}
                          </div>
                        </div>

                        <div className="flex items-center gap-xs">
                          {edition.formats.map((fmt) => (
                            <span key={fmt} className="book-edition-pill uppercase">
                              {fmt}
                            </span>
                          ))}
                        </div>

                        <div className="hidden sm:flex items-center gap-xs w-36">
                          <span className="size-1.5 rounded-full bg-success flex-none" />
                          <span className="text-xs text-text-2 truncate">Available</span>
                        </div>

                        {hasEpub && (
                          <Link
                            to={`/read/${work.id}/${edition.id}`}
                            className={cx('text-xs font-medium text-accent hover:underline cursor-pointer', FOCUS_RING)}
                          >
                            Read
                          </Link>
                        )}
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
              className="pt-lg flex flex-col gap-md outline-none"
            >
              {ownedCount > 0 ? (
                <div className="flex flex-col gap-sm">
                  {work.ownedEditions.map((ed) => (
                    <div
                      key={ed.id}
                      className="book-sources-card"
                    >
                      <div className="flex items-center gap-md">
                        <div className="book-source-badge">
                          OPDS
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="text-sm font-medium text-text truncate">
                            {ed.publisher || 'Connected Source'}
                          </div>
                          <div className="font-mono text-3xs text-text-3 mt-4xs">
                            {ed.language} · Synchronized catalog
                          </div>
                        </div>
                        <div className="flex items-center gap-xs px-md py-4xs rounded-4xl bg-surface-2 border border-border">
                          <span className="size-1.5 rounded-full bg-success flex-none" />
                          <span className="font-mono text-3xs text-text-2">Connected</span>
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-xs mt-md pt-sm border-t border-border/60">
                        {ed.formats.map((fmt) => (
                          <div
                            key={fmt}
                            className="book-source-file-row"
                          >
                            <span className="font-mono text-3xs font-medium text-text uppercase">
                              {fmt}
                            </span>
                            <span className="font-mono text-3xs text-text-3">
                              Ready
                            </span>
                            <div className="flex-1" />
                            {fmt.toLowerCase() === 'epub' && (
                              <Link
                                to={`/read/${work.id}/${ed.id}`}
                                className={cx('text-xs font-medium text-accent hover:underline cursor-pointer', FOCUS_RING)}
                              >
                                Read
                              </Link>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="p-xl rounded-md border border-dashed border-border text-center text-text-3 text-sm">
                  Not available from any connected sources.
                </div>
              )}

              <div className="book-sources-banner">
                <span className="size-1.5 rounded-full bg-text-3 flex-none" />
                <span className="flex-1">Manage connected sources and storage locations.</span>
                <Link to="/sources" className={cx('text-accent hover:underline cursor-pointer', FOCUS_RING)}>
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
