import { NavLink } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { TAB_ITEMS } from './navItems'
import {
  BookOpenIcon,
  CompassIcon,
  FolderIcon,
  SettingsIcon,
} from '../../components/Icon'

function getTabIcon(to: string) {
  switch (to) {
    case '/library':
      return <BookOpenIcon className="size-4 shrink-0" />
    case '/discover':
      return <CompassIcon className="size-4 shrink-0" />
    case '/collections':
      return <FolderIcon className="size-4 shrink-0" />
    case '/more':
      return <SettingsIcon className="size-4 shrink-0" />
    default:
      return null
  }
}

/**
 * Replaces the sidebar below the reflow breakpoint, matching the Mobile
 * layout's four-destination tab bar. A real <nav> landmark, same as the
 * sidebar.
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
                  // min-h-11 (44px): 44x44 touch-target minimum — py-sm alone measured 33px tall
                  'flex flex-col items-center justify-center gap-4xs py-sm min-h-11 text-3xs font-ui transition-colors',
                  isActive ? 'text-text font-semibold' : 'text-text-3 hover:text-text',
                  FOCUS_RING,
                )
              }
            >
              {getTabIcon(item.to)}
              <span>{item.label}</span>
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
