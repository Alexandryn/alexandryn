import { useEffect, useRef } from 'react'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { QueryResult } from '../../components/ErrorState'
import { useLibraryItems } from '../../data/library'

/**
 * The first screen wired to a data hook (frontend-shell-and-routing.md
 * FR-8): a genuinely empty successful response renders <EmptyState>, not
 * a blank content pane — distinct from the loading and error states
 * QueryResult already covers.
 */
export function Library() {
  const query = useLibraryItems()
  const headingRef = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    headingRef.current?.focus()
  }, [])

  return (
    <div className="p-3xl">
      <h1 ref={headingRef} tabIndex={-1} className="text-3xl font-medium tracking-1 outline-none">
        Library
      </h1>

      <div className="mt-lg">
        <QueryResult query={query} loadingLabel="Loading your library">
          {(items) =>
            items.length === 0 ? (
              <EmptyState
                title="Your library is waiting."
                description="Discover a book, connect a source, or import your existing collection."
              />
            ) : (
              <ul className="flex flex-col gap-xs">
                {items.map((item) => (
                  <li key={item.id} className="text-lg text-text">
                    <span className="font-medium">{item.title}</span>
                    <span className="text-text-2"> · {item.author}</span>
                  </li>
                ))}
              </ul>
            )
          }
        </QueryResult>
      </div>
    </div>
  )
}
