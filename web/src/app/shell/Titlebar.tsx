import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { getCurrentUser, logout } from '../../data/auth'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { AlexAvatar } from '../../components/Mascot'
import { SearchIcon } from '../../components/Icon'

/**
 * The persistent top bar: the wordmark with Alex the Cat avatar and global search.
 * A real <header> landmark.
 */
export function Titlebar() {
  const navigate = useNavigate()
  const user = getCurrentUser()
  const [searchQuery, setSearchQuery] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
        inputRef.current?.select()
        return
      }

      if (
        e.key === '/' &&
        !(
          e.target instanceof HTMLInputElement ||
          e.target instanceof HTMLTextAreaElement ||
          (e.target as HTMLElement)?.isContentEditable
        )
      ) {
        e.preventDefault()
        inputRef.current?.focus()
        inputRef.current?.select()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [])

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault()
    const trimmed = searchQuery.trim()
    if (trimmed) {
      navigate(`/discover?q=${encodeURIComponent(trimmed)}`)
    } else {
      navigate('/discover')
    }
  }

  return (
    <header className="flex h-[var(--shell-titlebar-height)] flex-none items-center gap-2xl border-b border-border bg-background px-xl select-none">
      {/* Wordmark with colored Alex the Cat Mascot */}
      <Link
        to="/library"
        className={cx(
          'flex items-center gap-xs focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent rounded-xs',
        )}
      >
        <AlexAvatar size="sm" />
        <span className="font-mono text-2xs uppercase tracking-11 text-text-2">
          ALEXANDRYN
        </span>
      </Link>

      {/* Centered Global Search Input */}
      <div className="flex flex-1 justify-center">
        <form
          role="search"
          onSubmit={handleSearch}
          className="w-full max-w-[var(--shell-search-width)]"
        >
          <label htmlFor="global-search-input" className="sr-only">
            Search library, authors, subjects, ISBN
          </label>
          <div
            className={cx(
              'group relative flex h-[var(--shell-search-height)] items-center rounded-2xs border border-border bg-surface px-md transition-colors',
              'focus-within:border-accent shadow-sm',
            )}
          >
            <SearchIcon className="size-3.5 text-text-3 pointer-events-none shrink-0 mr-2" />
            <input
              id="global-search-input"
              ref={inputRef}
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search library, authors, subjects, ISBN…"
              className="flex-1 bg-transparent border-none p-0 text-lg text-text placeholder:text-text-3 focus:outline-none"
            />
            <kbd
              aria-hidden="true"
              className="pointer-events-none hidden sm:inline-flex items-center rounded-4xs border border-border bg-surface px-1.5 py-0.5 font-mono text-3xs text-text-3 font-medium select-none"
            >
              ⌘K
            </kbd>
          </div>
        </form>
      </div>

      {/* Right Controls: Hosting pill & Profile / Auth */}
      <div className="flex items-center gap-md">
        <Link
          to="/network"
          className={cx(
            'hidden lg:flex items-center gap-2xs shell-hosting-pill px-md border border-border bg-surface text-text-2 hover:text-text transition-colors cursor-pointer',
            FOCUS_RING,
          )}
          title="Network hosting status"
        >
          <span className="size-1.5 rounded-full bg-success flex-none" />
          <span className="font-mono text-2xs uppercase tracking-7">
            HOSTING · 2 DEVICES
          </span>
        </Link>

        {user && (
          <div className="flex items-center gap-xs">
            <div
              title={user.username}
              className="shell-badge-circle bg-surface-3 flex items-center justify-center font-ui text-2xs font-medium text-text-2 uppercase select-none"
            >
              {user.username.slice(0, 2)}
            </div>
            <button
              onClick={handleLogout}
              className={cx(
                'rounded-3xs border border-border bg-surface px-sm py-2xs text-xs text-text-2 hover:text-text hover:bg-surface-2 transition-colors cursor-pointer',
                FOCUS_RING,
              )}
            >
              Sign Out
            </button>
          </div>
        )}
      </div>
    </header>
  )
}
