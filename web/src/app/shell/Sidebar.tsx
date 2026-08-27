import { NavLink } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { NAV_ITEMS } from './navItems'

const LINK_BASE = cx(
  'flex items-center h-3xl px-md rounded-2xs text-2xl font-ui',
  'text-text-2 hover:text-text hover:bg-surface-3',
  FOCUS_RING,
)

/**
 * The persistent desktop navigation rail
 * (frontend-shell-and-routing.md FR-3). A real <nav> landmark, not a
 * <div> with role — constitution §7. Rendered only at/above the reflow
 * breakpoint; below it the <MobileTabBar> replaces it entirely.
 */
export function Sidebar() {
  return (
    <nav
      aria-label="Primary"
      className="w-[var(--shell-sidebar-width)] shrink-0 border-r border-border bg-surface px-lg py-xl"
    >
      <ul className="flex flex-col gap-4xs">
        {NAV_ITEMS.map((item) => (
          <li
            key={item.to}
            className={cx(item.dividerBefore && 'mt-md border-t border-border pt-md')}
          >
            <NavLink
              to={item.to}
              className={({ isActive }) =>
                cx(LINK_BASE, isActive && 'bg-surface-3 text-text font-medium')
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
