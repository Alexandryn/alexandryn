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
    <nav aria-label="Primary" className="shrink-0 border-t border-border bg-surface">
      <ul className="flex">
        {TAB_ITEMS.map((item) => (
          <li key={item.to} className="flex-1">
            <NavLink
              to={item.to}
              className={({ isActive }) =>
                cx(
                  // min-h-11 (44px): 44x44 touch-target minimum (audit 0017 A-17-10) — py-sm alone measured 33px tall
                  'flex flex-col items-center justify-center gap-4xs py-sm min-h-11 text-3xs font-ui',
                  isActive ? 'text-accent font-medium' : 'text-text-3',
                  FOCUS_RING,
                )
              }
            >
              {item.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
