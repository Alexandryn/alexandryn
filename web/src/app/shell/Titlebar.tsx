import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getCurrentUser, logout } from '../../data/auth'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { LibrarySwitcher } from '../../screens/Libraries/LibrarySwitcher'
import { AlexAvatar } from '../../components/Mascot'
import { SearchIcon } from '../../components/Icon'

/**
 * The persistent top bar: the wordmark with Alex the Cat avatar, library switcher, and global search.
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
    <header className="flex items-center justify-between gap-lg border-b border-border bg-surface px-lg py-md">
      <div className="flex items-center gap-lg flex-1">
        <div className="flex items-center gap-xs select-none">
          <AlexAvatar size="sm" />
          <span className="font-ui font-semibold text-sm tracking-wider text-text">ALEXANDRYN</span>
        </div>
        <LibrarySwitcher />
        <form role="search" onSubmit={handleSearch} className="flex flex-1 max-w-[28rem]">
          <label htmlFor="global-search-input" className="sr-only">
            Search library, authors, subjects, ISBN
          </label>
          <div className="relative flex flex-1 items-center">
            <SearchIcon className="absolute left-3 size-4 text-text-3 pointer-events-none" />
            <input
              id="global-search-input"
              ref={inputRef}
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search library, authors, subjects, ISBN"
              className={cx(
                'flex flex-1 items-center h-9 w-full pl-9 pr-12',
                'rounded-xs border border-border bg-surface-2 text-sm text-text placeholder:text-text-3 transition-colors focus:bg-surface',
                FOCUS_RING,
              )}
            />
            <kbd
              aria-hidden="true"
              className="pointer-events-none absolute right-2.5 hidden sm:inline-flex items-center gap-1 rounded-4xs border border-border bg-surface px-1.5 py-0.5 font-mono text-3xs text-text-3 font-medium select-none shadow-2xs"
            >
              ⌘K
            </kbd>
          </div>
        </form>
      </div>

      {user && (
        <div className="flex items-center gap-md text-sm text-text-2">
          <span className="font-medium text-text">{user.username}</span>
          <button
            onClick={handleLogout}
            className={cx(
              'rounded-3xs border border-border px-sm py-2xs text-xs text-text-2 hover:text-text hover:bg-surface-3 transition-colors cursor-pointer',
              FOCUS_RING,
            )}
          >
            Sign Out
          </button>
        </div>
      )}
    </header>
  )
}
