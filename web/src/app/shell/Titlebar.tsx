import { Link } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

/**
 * The persistent top bar (frontend-shell-and-routing.md FR-3): the
 * wordmark and the global search affordance. A real <header> landmark
 * (constitution §7). The design reference's hosting-status pill and theme
 * toggle are deferred — hosting is phase 12/13, and Tier 1 shipped no
 * dark-mode toggle by decision (frontend-design-tokens.md FR-3).
 */
export function Titlebar() {
  return (
    <header className="flex items-center gap-lg border-b border-border bg-surface px-lg py-sm">
      <span className="font-mono text-2xs tracking-11 text-text-2">ALEXANDRYN</span>
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
    </header>
  )
}
