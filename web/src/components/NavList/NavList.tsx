import { Link } from 'react-router-dom'
import { ChevronRightIcon } from '../Icon'
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
 * screen both need.
 */
export function NavList({ items, ariaLabel }: { items: NavListItem[]; ariaLabel: string }) {
  return (
    <nav aria-label={ariaLabel}>
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface shadow-xs">
        {items.map((item) => (
          <li key={item.to}>
            <Link
              to={item.to}
              className={cx(
                'group flex items-center justify-between gap-md px-lg py-md transition-colors hover:bg-surface-2',
                FOCUS_RING,
              )}
            >
              <div className="flex flex-col gap-4xs">
                <span className="text-lg font-medium text-text">{item.label}</span>
                {item.description ? (
                  <span className="text-sm text-text-2">{item.description}</span>
                ) : null}
              </div>
              <ChevronRightIcon
                className="size-4 shrink-0 text-text-3 transition-transform group-hover:translate-x-0.5"
                aria-hidden="true"
              />
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  )
}
