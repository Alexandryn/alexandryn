import { Link } from 'react-router-dom'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

export interface NavListItem {
  to: string
  label: string
  description?: string
}

/**
 * A vertical list of navigation links with an optional one-line
 * description each — the shape the /settings index and the mobile /more
 * screen both need (audit 0016 #93).
 */
export function NavList({ items, ariaLabel }: { items: NavListItem[]; ariaLabel: string }) {
  return (
    <nav aria-label={ariaLabel}>
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
        {items.map((item) => (
          <li key={item.to}>
            <Link
              to={item.to}
              className={cx(
                'flex flex-col gap-4xs px-lg py-md hover:bg-surface-2',
                FOCUS_RING,
              )}
            >
              <span className="text-lg text-text">{item.label}</span>
              {item.description ? (
                <span className="text-sm text-text-2">{item.description}</span>
              ) : null}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  )
}
