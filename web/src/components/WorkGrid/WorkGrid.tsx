import { useEffect, useRef, useState, type HTMLAttributes } from 'react'
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
export const BATCH_SIZE = 20

function formatCoverAuthor(authors?: string[]): string | undefined {
  if (!authors || authors.length === 0) return undefined
  if (authors.length === 1) return authors[0]
  return `${authors[0]} et al.`
}

/**
 * Shared component for rendering a list of works as a responsive grid of
 * covers or a detailed list (frontend-library-screens.md FR-1, FR-4,
 * frontend-collections-screens.md FR-2).
 *
 * Virtualizes rendering above 100 items so only visible cover components mount.
 */
export function WorkGrid({ works, view, className, ...rest }: WorkGridProps) {
  const isVirtualized = works.length > VIRTUALIZATION_THRESHOLD
  const [mountedCount, setMountedCount] = useState<number>(INITIAL_VISIBLE_COUNT)
  const effectiveMountedCount = isVirtualized ? mountedCount : works.length
  const observerTargetRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!isVirtualized || effectiveMountedCount >= works.length) return
    if (typeof IntersectionObserver === 'undefined') return

    const target = observerTargetRef.current
    if (!target) return

    const observer = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting) {
        setMountedCount((prev) => Math.min(prev + BATCH_SIZE, works.length))
      }
    })

    observer.observe(target)
    return () => observer.disconnect()
  }, [isVirtualized, effectiveMountedCount, works.length])

  if (view === 'list') {
    return (
      <div className={cx('flex flex-col gap-xs', className)} {...rest}>
        <ul className="flex flex-col gap-xs">
          {works.map((work, index) => {
            const shouldMountCover = !isVirtualized || index < effectiveMountedCount
            const isObserverTarget = isVirtualized && index === effectiveMountedCount - 1
            return (
              <li
                key={work.id}
                ref={isObserverTarget ? (el) => { observerTargetRef.current = el } : undefined}
              >
                <Link
                  to={`/book/${work.id}`}
                  className={cx(
                    'group flex items-center justify-between gap-md rounded-xs border border-border bg-surface p-sm transition-colors hover:border-text-3 hover:bg-surface-2',
                    FOCUS_RING,
                  )}
                >
                  <div className="flex items-center gap-md min-w-0">
                    <div className="w-8 shrink-0 aspect-[2/3] overflow-hidden rounded-3xs bg-surface-3">
                      {shouldMountCover ? (
                        <GeneratedCover
                          identifier={work.id}
                          title={work.title}
                          author={formatCoverAuthor(work.authors)}
                        />
                      ) : (
                        <div
                          data-testid="virtual-cover-placeholder"
                          className="size-full bg-surface-3"
                          aria-hidden="true"
                        />
                      )}
                    </div>
                    <div className="flex flex-col min-w-0">
                      <div className="flex items-baseline gap-xs">
                        <span className="font-medium text-text text-sm truncate group-hover:text-primary">
                          {work.title}
                        </span>
                        {work.subtitle ? (
                          <span className="text-text-2 text-xs truncate">
                            {work.subtitle}
                          </span>
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
          const shouldMountCover = !isVirtualized || index < effectiveMountedCount
          const isObserverTarget = isVirtualized && index === effectiveMountedCount - 1
          return (
            <div
              key={work.id}
              ref={isObserverTarget ? (el) => { observerTargetRef.current = el } : undefined}
            >
              <Link
                to={`/book/${work.id}`}
                className={cx(
                  'group flex flex-col rounded-xs p-xs transition-colors hover:bg-surface-2',
                  FOCUS_RING,
                )}
              >
                <div className="aspect-[2/3] w-full overflow-hidden rounded-xs bg-surface-3 shadow-sm group-hover:shadow-md transition-shadow">
                  {shouldMountCover ? (
                    <GeneratedCover
                      identifier={work.id}
                      title={work.title}
                      author={formatCoverAuthor(work.authors)}
                    />
                  ) : (
                    <div
                      data-testid="virtual-cover-placeholder"
                      className="size-full bg-surface-3"
                      aria-hidden="true"
                    />
                  )}
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
