import { Link, useNavigate } from 'react-router-dom'
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

  const handleLogout = async () => {
    await logout()
    navigate('/login')
  }

  return (
    <header className="flex items-center justify-between gap-lg border-b border-border bg-surface px-lg py-sm">
      <div className="flex items-center gap-lg flex-1">
        <span className="font-mono text-2xs tracking-11 text-text-2">ALEXANDRYN</span>
        <LibrarySwitcher />
        <Link
          to="/discover"
          className={cx(
            'flex flex-1 items-center gap-md h-3xl max-w-[28rem] px-md',
            'rounded-2xs border border-border bg-surface-2 text-lg text-text-3',
            FOCUS_RING,
          )}
        >
          Search library, authors, subjects, ISBN
        </Link>
      </div>

      {user && (
        <div className="flex items-center gap-md text-xs text-text-2">
          <span>{user.username}</span>
          <button
            onClick={handleLogout}
            className="text-text-3 hover:text-text-1 underline cursor-pointer"
          >
            Sign Out
          </button>
        </div>
      )}
    </header>
  )
}

