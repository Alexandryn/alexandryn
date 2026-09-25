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
 * Above 100 items, cards inside a sliding window of WINDOW_SIZE are fully
 * mounted (including Link, GeneratedCover, and metadata subtrees); off-screen
 * items render lightweight placeholders that match card geometry to prevent
 * layout shift while avoiding linear DOM tree depth and React fiber cost.
 * The window slides off two IntersectionObserver sentinels for smooth scrolling,
 * and recovers via scroll tracking when the user jumps or drags the scrollbar.
 */
export function WorkGrid({ works, view, className, ...rest }: WorkGridProps) {
  const isVirtualized = works.length > VIRTUALIZATION_THRESHOLD

  const [windowStart, setWindowStart] = useState(0)
  const maxStart = Math.max(0, works.length - WINDOW_SIZE)
  const clampedStart = Math.min(windowStart, maxStart)
  const windowEnd = isVirtualized
    ? Math.min(works.length, clampedStart + WINDOW_SIZE)
    : works.length

  const rootRef = useRef<HTMLDivElement | null>(null)
  const topSentinelRef = useRef<HTMLElement | null>(null)
  const bottomSentinelRef = useRef<HTMLElement | null>(null)
  const lastScrollY = useRef(0)

  // Smooth sentinel-driven sliding window
  useEffect(() => {
    if (!isVirtualized) return
    if (typeof IntersectionObserver === 'undefined') return

    const observer = new IntersectionObserver(
      (entries) => {
        let bottomHit = false
        let topHit = false
        for (const entry of entries) {
          if (!entry.isIntersecting) continue
          if (entry.target === bottomSentinelRef.current) bottomHit = true
          if (entry.target === topSentinelRef.current) topHit = true
        }

        if (bottomHit && topHit) {
          // Deadlock guard: on huge viewports or zoom-out where both sentinels are visible,
          // only advance in the direction of active scroll; do not oscillate or cancel out.
          const scrollY = typeof window !== 'undefined' ? window.scrollY : 0
          const scrollingDown = scrollY > lastScrollY.current
          const scrollingUp = scrollY < lastScrollY.current
          lastScrollY.current = scrollY
          if (scrollingDown) {
            setWindowStart((s) => Math.min(maxStart, s + BATCH_SIZE))
          } else if (scrollingUp) {
            setWindowStart((s) => Math.max(0, s - BATCH_SIZE))
          }
        } else if (bottomHit) {
          if (typeof window !== 'undefined') lastScrollY.current = window.scrollY
          setWindowStart((s) => Math.min(maxStart, s + BATCH_SIZE))
        } else if (topHit) {
          if (typeof window !== 'undefined') lastScrollY.current = window.scrollY
          setWindowStart((s) => Math.max(0, s - BATCH_SIZE))
        }
      },
      { rootMargin: `${SENTINEL_ROOT_MARGIN_PX}px` },
    )

    if (topSentinelRef.current) observer.observe(topSentinelRef.current)
    if (bottomSentinelRef.current) observer.observe(bottomSentinelRef.current)
    return () => observer.disconnect()
  }, [isVirtualized, maxStart, clampedStart, windowEnd])

  // Scrollbar drag & large jump recovery
  useEffect(() => {
    if (!isVirtualized) return
    if (typeof window === 'undefined') return

    let rafId: number | null = null
    const handleScroll = () => {
      if (rafId !== null) return
      rafId = window.requestAnimationFrame(() => {
        rafId = null
        const el = rootRef.current
        if (!el) return
        const rect = el.getBoundingClientRect()
        const viewportHeight = window.innerHeight || 1
        if (rect.height <= 0) return

        const scrolled = Math.max(0, -rect.top)
        const scrollable = Math.max(1, rect.height - viewportHeight)
        const ratio = Math.min(1, Math.max(0, scrolled / scrollable))

        const centerIndex = Math.floor(ratio * (works.length - 1))
        const targetStart = Math.max(0, Math.min(maxStart, Math.round(centerIndex - WINDOW_SIZE / 2)))

        // If the window has jumped outside the sliding window (e.g. scrollbar drag),
        // resync windowStart to keep visible cards mounted.
        setWindowStart((curr) => {
          if (Math.abs(curr - targetStart) > WINDOW_SIZE) {
            return targetStart
          }
          return curr
        })
      })
    }

    window.addEventListener('scroll', handleScroll, { passive: true })
    return () => {
      window.removeEventListener('scroll', handleScroll)
      if (rafId !== null) window.cancelAnimationFrame(rafId)
    }
  }, [isVirtualized, maxStart, works.length])

  const isItemMounted = (index: number) =>
    !isVirtualized || (index >= clampedStart && index < windowEnd)

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
  const sentinelRef = useCallback(
    (kind: 'top' | 'bottom' | undefined) =>
      kind === 'top' ? setTopSentinel : kind === 'bottom' ? setBottomSentinel : undefined,
    [setTopSentinel, setBottomSentinel],
  )

  if (view === 'list') {
    return (
      <div ref={rootRef} className={cx('flex flex-col gap-xs', className)} {...rest}>
        <ul className="flex flex-col gap-xs">
          {works.map((work, index) => {
            const sentinel = sentinelKind(index)
            if (!isItemMounted(index)) {
              return (
                <li
                  key={work.id}
                  ref={sentinel ? sentinelRef(sentinel) : undefined}
                  data-sentinel={sentinel}
                  className="cv-auto-list"
                  aria-hidden="true"
                >
                  <div
                    data-testid="virtual-cover-placeholder"
                    className="h-16 w-full rounded-xs bg-surface-2/40"
                  />
                </li>
              )
            }

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
                      <GeneratedCover
                        identifier={work.id}
                        title={work.title}
                        author={formatCoverAuthor(work.authors)}
                      />
                    </div>
                    <div className="flex flex-col min-w-0">
                      <div className="flex items-baseline gap-xs">
                        <span className="font-medium text-text text-sm truncate group-hover:text-primary">
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
    <div ref={rootRef} className={cx('flex flex-col gap-md', className)} {...rest}>
      <div className="grid grid-cols-2 gap-md sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {works.map((work, index) => {
          const sentinel = sentinelKind(index)
          if (!isItemMounted(index)) {
            return (
              <div
                key={work.id}
                ref={sentinel ? sentinelRef(sentinel) : undefined}
                data-sentinel={sentinel}
                className="cv-auto"
                aria-hidden="true"
              >
                <div className="flex flex-col rounded-xs p-xs">
                  <div
                    data-testid="virtual-cover-placeholder"
                    className="aspect-[2/3] w-full rounded-xs bg-surface-3"
                  />
                  <div className="mt-xs flex flex-col gap-4xs" aria-hidden="true">
                    <div className="h-4 w-3/4 rounded-3xs bg-surface-2/40" />
                    <div className="h-3 w-1/2 rounded-3xs bg-surface-2/30" />
                  </div>
                </div>
              </div>
            )
          }

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
                  <GeneratedCover
                    identifier={work.id}
                    title={work.title}
                    author={formatCoverAuthor(work.authors)}
                  />
                </div>

                <div className="mt-xs flex flex-col">
                  <span className="font-medium text-sm text-text line-clamp-2 leading-snug group-hover:text-primary">
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
