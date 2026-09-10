import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getCurrentUser, logout } from '../../data/auth'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { LibrarySwitcher } from '../../screens/Libraries/LibrarySwitcher'

/**
 * The persistent top bar (frontend-shell-and-routing.md FR-3): the
 * wordmark, library switcher, and the global search affordance. A real <header> landmark
 * (constitution §7).
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
        <span className="font-mono text-2xs tracking-11 text-text-2">ALEXANDRYN</span>
        <LibrarySwitcher />
        <form role="search" onSubmit={handleSearch} className="flex flex-1 max-w-[28rem]">
          <label htmlFor="global-search-input" className="sr-only">
            Search library, authors, subjects, ISBN
          </label>
          <input
            id="global-search-input"
            type="search"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search library, authors, subjects, ISBN"
            className={cx(
              'flex flex-1 items-center gap-md h-3xl w-full px-md',
              'rounded-2xs border border-border bg-surface-2 text-sm text-text placeholder:text-text-3',
              FOCUS_RING,
            )}
          />
        </form>
      </div>

      {user && (
        <div className="flex items-center gap-md text-xs text-text-2">
          <span>{user.username}</span>
          <button
            onClick={handleLogout}
            className="text-text-3 hover:text-text underline cursor-pointer"
          >
            Sign Out
          </button>
        </div>
      )}
    </header>
  )
}
