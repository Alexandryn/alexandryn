import { NavLink } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { TAB_ITEMS } from './navItems'

/**
 * Replaces the sidebar below the reflow breakpoint
 * (frontend-shell-and-routing.md FR-3), matching the Mobile canvas's
 * four-destination tab bar. A real <nav> landmark, same as the sidebar.
 */
export function MobileTabBar() {
  return (
    <nav aria-label="Primary" className="flex shrink-0 border-t border-border bg-surface">
      {TAB_ITEMS.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          className={({ isActive }) =>
            cx(
              'flex flex-1 flex-col items-center gap-4xs py-sm text-3xs font-ui',
              isActive ? 'text-accent font-medium' : 'text-text-3',
              FOCUS_RING,
            )
          }
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  )
}
