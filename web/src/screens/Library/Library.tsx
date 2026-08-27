import { Link } from 'react-router-dom'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { QueryResult } from '../../components/ErrorState'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { useLibraryItems } from '../../data/library'

/**
 * The first screen wired to a data hook (frontend-shell-and-routing.md
 * FR-8): a genuinely empty successful response renders <EmptyState>, not
 * a blank content pane — distinct from the loading and error states
 * QueryResult already covers.
 */
export function Library() {
  const query = useLibraryItems()

  return (
    <div className="p-3xl">
      <h1 className="text-3xl font-medium tracking-1">Library</h1>

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
                  <li key={item.id}>
                    <Link
                      to={`/book/${item.id}`}
                      className={cx('inline-block rounded-2xs text-lg text-text', FOCUS_RING)}
                    >
                      <span className="font-medium">{item.title}</span>
                      <span className="text-text-2"> · {item.author}</span>
                    </Link>
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
