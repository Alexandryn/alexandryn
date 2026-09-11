import { useCallback, useEffect, useRef, useState, useTransition } from 'react'
import { useSearchParams } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { DiscoverResultGrid } from '../../components/DiscoverResultGrid'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Input } from '../../components/Input/Input'
import { Spinner } from '../../components/Spinner/Spinner'
import { useDiscoverSearch } from '../../data/discover'
import { ApiError } from '../../data/http'

const SEARCH_DEBOUNCE_MS = 300
const PAGE_SIZE = 20

export function Discover() {
  const [searchParams, setSearchParams] = useSearchParams()
  const [, startTransition] = useTransition()

  // 1. URL search params (FR-2)
  const qParam = searchParams.get('q') ?? ''
  const offsetParam = parseInt(searchParams.get('offset') ?? '0', 10)
  const offset = Number.isNaN(offsetParam) || offsetParam < 0 ? 0 : offsetParam

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
              // Reset offset to 0 on new query
              next.delete('offset')
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

  // 3. Query execution (FR-1, FR-3)
  const { data, error, isPending, refetch } = useDiscoverSearch({
    q: qParam,
    limit: PAGE_SIZE,
    offset,
  })

  const items = data?.items ?? []
  const total = data?.total ?? 0

  // 4. Pagination handlers (FR-3)
  // Paging swaps the entire result list, so the Previous/Next button that
  // was clicked unmounts and focus falls to the document body. After the
  // new page settles, move focus to the results region so keyboard and
  // screen-reader users keep their place (audit 0016 #152).
  const resultsRef = useRef<HTMLDivElement>(null)
  const headingRef = useRef<HTMLHeadingElement>(null)
  const focusResultsOnPage = useRef(false)

  useEffect(() => {
    if (!focusResultsOnPage.current || isPending) return
    focusResultsOnPage.current = false
    // Prefer the results region; fall back to the page heading when the
    // new page rendered an error or empty state instead, so focus never
    // drops to the document body.
    ;(resultsRef.current ?? headingRef.current)?.focus()
  }, [offset, isPending])

  const handlePreviousPage = () => {
    focusResultsOnPage.current = true
    const nextOffset = Math.max(0, offset - PAGE_SIZE)
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      if (nextOffset > 0) {
        next.set('offset', String(nextOffset))
      } else {
        next.delete('offset')
      }
      return next
    })
  }

  const handleNextPage = () => {
    focusResultsOnPage.current = true
    const nextOffset = offset + PAGE_SIZE
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev)
      next.set('offset', String(nextOffset))
      return next
    })
  }

  const hasPreviousPage = offset > 0
  const hasNextPage = offset + PAGE_SIZE < total

  return (
    <div className="flex flex-col gap-xl p-3xl">
      {/* Header */}
      <div className="flex flex-col gap-md">
        <h1
          ref={headingRef}
          tabIndex={-1}
          className="text-3xl font-medium tracking-1 text-text outline-none"
        >
          Discover
        </h1>

        {/* Search Input (FR-1) */}
        {/* max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key (audit 0017 A-17-08) */}
        <div className="w-full max-w-[28rem]">
          <Input
            label="Search Open Library"
            placeholder="Search titles, authors..."
            value={searchInputValue}
            onChange={handleSearchChange}
          />
        </div>
      </div>

      {/* Screen reader live announcement (FR-1 / a11y) */}
      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {!isPending && qParam && `${total} results found for "${qParam}"`}
      </div>

      {/* Content Area */}
      <div>
        {!qParam ? (
          <EmptyState
            title="Search Open Library"
            description="Search millions of books by title, author, or subject to discover your next read."
          />
        ) : isPending ? (
          <Spinner label="Searching Open Library" className="m-3xl" />
        ) : error ? (
          <ErrorState
            title={
              error instanceof ApiError &&
              (error.status === 503 || error.code.toLowerCase() === 'unavailable')
                ? 'Open Library is unavailable right now, try again shortly'
                : 'Something went wrong searching Open Library'
            }
            description={error instanceof ApiError ? error.message : undefined}
            code={error instanceof ApiError ? error.code : undefined}
            correlationId={error instanceof ApiError ? error.correlationId : undefined}
            onRetry={() => void refetch()}
          />
        ) : items.length === 0 ? (
          <EmptyState
            title="No results found"
            description={`No books matched "${qParam}". Try another search term.`}
          />
        ) : (
          <div
            ref={resultsRef}
            tabIndex={-1}
            role="region"
            aria-label={`Search results, showing ${offset + 1} to ${Math.min(
              offset + PAGE_SIZE,
              total,
            )} of ${total}`}
            className="flex flex-col gap-lg outline-none"
          >
            <DiscoverResultGrid results={items} />

            {/* Pagination Controls (FR-3) */}
            <div className="flex flex-wrap items-center justify-between gap-md border-t border-border pt-md">
              <span className="text-xs text-text-2">
                Showing {offset + 1} to {Math.min(offset + PAGE_SIZE, total)} of {total} results
              </span>

              <div className="flex items-center gap-xs">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handlePreviousPage}
                  disabled={!hasPreviousPage || isPending}
                  aria-label="Previous page"
                >
                  Previous
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleNextPage}
                  disabled={!hasNextPage || isPending}
                  aria-label="Next page"
                >
                  Next
                </Button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
