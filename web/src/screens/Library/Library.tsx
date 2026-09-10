import { useCallback, useEffect, useMemo, useRef, useState, useTransition } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Input } from '../../components/Input/Input'
import { SegmentedControl } from '../../components/SegmentedControl/SegmentedControl'
import { Spinner } from '../../components/Spinner/Spinner'
import { WorkGrid } from '../../components/WorkGrid'
import {
  useLibrary,
  type LibraryFilter,
  type LibrarySort,
  type WorkSummary,
} from '../../data/library'
import { ApiError } from '../../data/http'

const STORAGE_VIEW_KEY = 'alexandryn:library-view'
const SEARCH_DEBOUNCE_MS = 300

const FILTER_OPTIONS = [
  { value: 'all', label: 'All' },
  { value: 'owned', label: 'Owned' },
  { value: 'wanted', label: 'Wanted' },
]

const SORT_OPTIONS = [
  { value: 'added_at', label: 'Recently added' },
  { value: 'title', label: 'Title' },
]

const VIEW_OPTIONS = [
  { value: 'grid', label: 'Grid' },
  { value: 'list', label: 'List' },
]

export function Library() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [, startTransition] = useTransition()

  // 1. URL search params (FR-2)
  const qParam = searchParams.get('q') ?? ''
  const filterParam = (searchParams.get('filter') as LibraryFilter) || 'all'
  const sortParam = (searchParams.get('sort') as LibrarySort) || 'added_at'

  const activeFilter: LibraryFilter =
    filterParam === 'owned' || filterParam === 'wanted' ? filterParam : 'all'
  const activeSort: LibrarySort = sortParam === 'title' ? 'title' : 'added_at'

  // 2. Debounced search input state (FR-1)
  const [searchInputValue, setSearchInputValue] = useState(qParam)
  const [prevQ, setPrevQ] = useState(qParam)
  if (qParam !== prevQ) {
    setPrevQ(qParam)
    if (searchInputValue.trim() !== qParam.trim()) {
      setSearchInputValue(qParam)
    }
  }

  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleSearchChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const nextVal = e.target.value
      setSearchInputValue(nextVal)

      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }

      debounceTimerRef.current = setTimeout(() => {
        startTransition(() => {
          setSearchParams(
            (prev) => {
              const next = new URLSearchParams(prev)
              const trimmed = nextVal.trim()
              if (trimmed) {
                next.set('q', trimmed)
              } else {
                next.delete('q')
              }
              return next
            },
            { replace: true },
          )
        })
      }, SEARCH_DEBOUNCE_MS)
    },
    [setSearchParams],
  )

  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current)
      }
    }
  }, [])

  // 3. Filter & Sort handlers (FR-1, FR-2)
  const handleFilterChange = (val: string) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (val === 'all') {
        next.delete('filter')
      } else {
        next.set('filter', val)
      }
      return next
    })
  }

  const handleSortChange = (val: string) => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (val === 'added_at') {
        next.delete('sort')
      } else {
        next.set('sort', val)
      }
      return next
    })
  }

  // 4. View toggle preference (localStorage, FR-1, FR-2)
  const [view, setView] = useState<'grid' | 'list'>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_VIEW_KEY)
      return stored === 'list' ? 'list' : 'grid'
    } catch {
      return 'grid'
    }
  })

  const handleViewChange = (nextView: string) => {
    const v = nextView === 'list' ? 'list' : 'grid'
    setView(v)
    try {
      localStorage.setItem(STORAGE_VIEW_KEY, v)
    } catch {
      // Ignore storage write error
    }
  }

  // 5. Cursor-paginated Infinite Query (FR-3)
  const libraryQuery = useLibrary({
    q: qParam,
    filter: activeFilter,
    sort: activeSort,
  })

  const { data, error, isPending, isFetchingNextPage, hasNextPage, fetchNextPage, refetch } =
    libraryQuery

  // Memoize allWorks to preserve array identity across keystrokes/transitions (audit 0016 #217).
  const allWorks: WorkSummary[] = useMemo(
    () => data?.pages.flatMap((page) => page.works) ?? [],
    [data],
  )

  // 6. Infinite scroll sentinel intersection observer
  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!hasNextPage || isFetchingNextPage) return

    const target = sentinelRef.current
    if (!target) return

    if (typeof IntersectionObserver === 'undefined') return

    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) {
        void fetchNextPage()
      }
    })

    observer.observe(target)
    return () => observer.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  const hasActiveFilters = Boolean(qParam || activeFilter !== 'all' || activeSort !== 'added_at')

  return (
    <div className="flex flex-col gap-xl p-3xl">
      {/* Screen Header */}
      <div className="flex flex-col gap-md">
        <h1 className="text-3xl font-medium tracking-1 text-text">Library</h1>

        {/* Controls Bar: Search, Filters, Sort, View Toggle */}
        <div className="flex flex-wrap items-center justify-between gap-md">
          <div className="flex flex-wrap items-center gap-md flex-1 min-w-64">
            <div className="w-full max-w-xs">
              <Input
                label="Search library"
                placeholder="Search titles, authors..."
                value={searchInputValue}
                onChange={handleSearchChange}
                aria-label="Search library"
              />
            </div>

            <SegmentedControl
              options={FILTER_OPTIONS}
              value={activeFilter}
              onValueChange={handleFilterChange}
              aria-label="Filter library"
            />

            <SegmentedControl
              options={SORT_OPTIONS}
              value={activeSort}
              onValueChange={handleSortChange}
              aria-label="Sort library"
            />
          </div>

          <SegmentedControl
            options={VIEW_OPTIONS}
            value={view}
            onValueChange={handleViewChange}
            aria-label="Toggle view layout"
          />
        </div>
      </div>

      {/* Screen reader live announcement for newly loaded items (FR-1 / a11y) */}
      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {!isPending && `${allWorks.length} works loaded`}
      </div>

      {/* Content Area */}
      <div>
        {isPending ? (
          <Spinner label="Loading your library" className="m-3xl" />
        ) : error ? (
          <ErrorState
            title="Something went wrong loading your library"
            description={error instanceof ApiError ? error.message : undefined}
            code={error instanceof ApiError ? error.code : undefined}
            correlationId={error instanceof ApiError ? error.correlationId : undefined}
            onRetry={() => void refetch()}
          />
        ) : allWorks.length === 0 ? (
          hasActiveFilters ? (
            <EmptyState
              title="No books match your search"
              description="Try adjusting your keywords or clearing your filters."
              action={{
                label: 'Clear filters',
                onClick: () => setSearchParams(new URLSearchParams()),
              }}
            />
          ) : (
            <EmptyState
              title="Your library is waiting."
              description="Discover a book, connect a source, or import your existing collection."
            />
          )
        ) : (
          <div className="flex flex-col gap-lg">
            <WorkGrid works={allWorks} view={view} />

            {/* Infinite Scroll Sentinel / Load More Status */}
            {hasNextPage && (
              <div className="flex flex-col items-center justify-center p-md">
                <div ref={sentinelRef} className="h-4 w-full" aria-hidden="true" />
                {isFetchingNextPage ? (
                  <Spinner label="Loading more books" />
                ) : (
                  <Button variant="ghost" size="sm" onClick={() => void fetchNextPage()}>
                    Load more
                  </Button>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
