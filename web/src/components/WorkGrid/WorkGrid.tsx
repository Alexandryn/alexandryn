import { useCallback, useEffect, useRef, useState, type HTMLAttributes } from 'react'
import { Link } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { GeneratedCover } from '../GeneratedCover/GeneratedCover'
import type { WorkSummary } from '../../data/library'

export interface WorkGridProps extends HTMLAttributes<HTMLDivElement> {
  works: WorkSummary[]
  view: 'grid' | 'list'
}

export const VIRTUALIZATION_THRESHOLD = 100
export const INITIAL_VISIBLE_COUNT = 40
export const WINDOW_SIZE = INITIAL_VISIBLE_COUNT
export const BATCH_SIZE = 20
// How far outside the viewport a sentinel triggers the window slide. Not a
// style value — an IntersectionObserver margin — but kept numeric so the
// token-only styling check doesn't read the "300px" string as CSS.
const SENTINEL_ROOT_MARGIN_PX = 300

function formatCoverAuthor(authors?: string[]): string | undefined {
  if (!authors || authors.length === 0) return undefined
  if (authors.length === 1) return authors[0]
  return `${authors[0]} et al.`
}

/**
 * Shared component for rendering a list of works as a responsive grid of
 * covers or a detailed list.
 *
 * Above 100 items only the covers inside a sliding window of WINDOW_SIZE
 * are mounted; the rest render an aspect-ratio placeholder so the scroll
 * height is unchanged. The window slides both ways off two sentinels, so
 * a cover scrolled well out of view is unmounted again
 * — the number of live cover components stays bounded no matter how far
 * the user scrolls.
 */
export function WorkGrid({ works, view, className, ...rest }: WorkGridProps) {
  const isVirtualized = works.length > VIRTUALIZATION_THRESHOLD

  const [windowStart, setWindowStart] = useState(0)
  const maxStart = Math.max(0, works.length - WINDOW_SIZE)
  const clampedStart = Math.min(windowStart, maxStart)
  const windowEnd = isVirtualized
    ? Math.min(works.length, clampedStart + WINDOW_SIZE)
    : works.length

  const topSentinelRef = useRef<HTMLElement | null>(null)
  const bottomSentinelRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!isVirtualized) return
    if (typeof IntersectionObserver === 'undefined') return

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          if (entry.target === bottomSentinelRef.current) {
            setWindowStart((s) => Math.min(maxStart, s + BATCH_SIZE))
          } else if (entry.target === topSentinelRef.current) {
            setWindowStart((s) => Math.max(0, s - BATCH_SIZE))
          }
        }
      },
      { rootMargin: `${SENTINEL_ROOT_MARGIN_PX}px` },
    )

    if (topSentinelRef.current) observer.observe(topSentinelRef.current)
    if (bottomSentinelRef.current) observer.observe(bottomSentinelRef.current)
    return () => observer.disconnect()
  }, [isVirtualized, maxStart, clampedStart, windowEnd])

  const coverFor = (index: number) => !isVirtualized || (index >= clampedStart && index < windowEnd)

  const sentinelKind = (index: number): 'top' | 'bottom' | undefined => {
    if (!isVirtualized) return undefined
    if (clampedStart > 0 && index === clampedStart) return 'top'
    if (windowEnd < works.length && index === windowEnd - 1) return 'bottom'
    return undefined
  }
  const setTopSentinel = useCallback((el: HTMLElement | null) => {
    topSentinelRef.current = el
  }, [])
  const setBottomSentinel = useCallback((el: HTMLElement | null) => {
    bottomSentinelRef.current = el
  }, [])
  const sentinelRef = (kind: 'top' | 'bottom' | undefined) =>
    kind === 'top' ? setTopSentinel : kind === 'bottom' ? setBottomSentinel : undefined

  const coverPlaceholder = (
    <div
      data-testid="virtual-cover-placeholder"
      className="size-full bg-surface-3"
      aria-hidden="true"
    />
  )

  if (view === 'list') {
    return (
      <div className={cx('flex flex-col gap-xs', className)} {...rest}>
        <ul className="flex flex-col gap-xs">
          {works.map((work, index) => {
            const sentinel = sentinelKind(index)
            return (
              <li
                key={work.id}
                ref={sentinel ? sentinelRef(sentinel) : undefined}
                data-sentinel={sentinel}
                className="cv-auto-list"
              >
                <Link
                  to={`/book/${work.id}`}
                  className={cx(
                    'group flex items-center justify-between gap-md rounded-xs border border-border bg-surface p-sm transition-colors hover:border-text-3 hover:bg-surface-2',
                    FOCUS_RING,
                  )}
                >
                  <div className="flex items-center gap-md min-w-0">
                    <div className="relative w-8 shrink-0 aspect-[2/3] overflow-hidden rounded-3xs bg-surface-3 book-shadow">
                      <div className="absolute inset-y-0 left-0 w-1.5 book-spine-crease pointer-events-none z-10" />
                      {coverFor(index) ? (
                        <GeneratedCover
                          identifier={work.id}
                          title={work.title}
                          author={formatCoverAuthor(work.authors)}
                        />
                      ) : (
                        coverPlaceholder
                      )}
                    </div>
                    <div className="flex flex-col min-w-0">
                      <div className="flex items-baseline gap-xs">
                        <span className="font-medium text-text text-sm truncate">
                          {work.title}
                        </span>
                        {work.subtitle ? (
                          <span className="text-text-2 text-xs truncate">{work.subtitle}</span>
                        ) : null}
                      </div>
                      <span className="text-text-2 text-xs truncate">
                        {work.authors && work.authors.length > 0
                          ? work.authors.join(', ')
                          : 'Unknown Author'}
                      </span>
                    </div>
                  </div>

                  <div className="flex items-center gap-sm shrink-0">
                    {work.collections && work.collections.length > 0 && (
                      <div className="hidden sm:flex items-center gap-4xs">
                        {work.collections.map((c) => (
                          <span
                            key={c.id}
                            className="rounded-3xs bg-surface-3 px-2xs py-4xs text-3xs text-text-2"
                          >
                            {c.name}
                          </span>
                        ))}
                      </div>
                    )}
                    {!work.isOwned && (
                      <span className="rounded-3xs border border-border-2 px-2xs py-4xs text-3xs font-mono text-text-2 uppercase">
                        Wanted
                      </span>
                    )}
                  </div>
                </Link>
              </li>
            )
          })}
        </ul>
      </div>
    )
  }

  return (
    <div className={cx('flex flex-col gap-md', className)} {...rest}>
      <div className="grid grid-cols-2 gap-md sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {works.map((work, index) => {
          const sentinel = sentinelKind(index)
          return (
            <div
              key={work.id}
              ref={sentinel ? sentinelRef(sentinel) : undefined}
              data-sentinel={sentinel}
              className="cv-auto"
            >
              <Link
                to={`/book/${work.id}`}
                className={cx(
                  'group flex flex-col rounded-xs p-xs transition-colors hover:bg-surface-2',
                  FOCUS_RING,
                )}
              >
                <div className="relative aspect-[2/3] w-full overflow-hidden rounded-xs bg-surface-3 book-shadow group-hover:book-shadow-hover transition-all duration-200 group-hover:-translate-y-1">
                  <div className="absolute inset-y-0 left-0 w-3 book-spine-crease pointer-events-none z-10" />
                  {coverFor(index) ? (
                    <GeneratedCover
                      identifier={work.id}
                      title={work.title}
                      author={formatCoverAuthor(work.authors)}
                    />
                  ) : (
                    coverPlaceholder
                  )}
                </div>

                <div className="mt-xs flex flex-col">
                  <span className="font-medium text-sm text-text line-clamp-2 leading-snug">
                    {work.title}
                  </span>
                  <span className="text-xs text-text-2 line-clamp-1 mt-4xs">
                    {work.authors && work.authors.length > 0
                      ? work.authors.join(', ')
                      : 'Unknown Author'}
                  </span>
                  {!work.isOwned && (
                    <span className="mt-xs inline-block self-start rounded-3xs border border-border-2 px-2xs py-4xs text-3xs font-mono text-text-2 uppercase">
                      Wanted
                    </span>
                  )}
                </div>
              </Link>
            </div>
          )
        })}
      </div>
    </div>
  )
}
