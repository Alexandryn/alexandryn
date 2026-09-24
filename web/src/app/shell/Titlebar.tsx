import { useState } from 'react'
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
    <header className="flex items-center justify-between gap-lg border-b border-border bg-surface px-lg py-sm">
      <div className="flex items-center gap-lg flex-1">
        <div className="flex items-center gap-xs select-none">
          <AlexAvatar size="sm" />
          <span className="font-ui font-semibold text-xs tracking-wide text-text">ALEXANDRYN</span>
        </div>
        <LibrarySwitcher />
        <form role="search" onSubmit={handleSearch} className="flex flex-1 max-w-[28rem]">
          <label htmlFor="global-search-input" className="sr-only">
            Search library, authors, subjects, ISBN
          </label>
          <div className="relative flex flex-1 items-center">
            <SearchIcon className="absolute left-md size-4 text-text-3 pointer-events-none" />
            <input
              id="global-search-input"
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search library, authors, subjects, ISBN"
              className={cx(
                'flex flex-1 items-center h-3xl w-full pl-3xl pr-md',
                'rounded-2xs border border-border bg-surface-2 text-sm text-text placeholder:text-text-3',
                FOCUS_RING,
              )}
            />
          </div>
        </form>
      </div>

      {user && (
        <div className="flex items-center gap-md text-xs text-text-2">
          <span className="font-medium text-text">{user.username}</span>
          <button
            onClick={handleLogout}
            className={cx(
              'rounded-3xs border border-border px-xs py-4xs text-2xs text-text-2 hover:text-text hover:bg-surface-3 transition-colors cursor-pointer',
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
