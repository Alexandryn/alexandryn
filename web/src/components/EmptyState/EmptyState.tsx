import type { HTMLAttributes, ReactNode } from 'react'
import { Button } from '../Button/Button'
import { cx } from '../../lib/cx'

export interface EmptyStateProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  title: string
  description?: string
  icon?: ReactNode
  action?: { label: string; onClick: () => void }
}

/** Composed from an icon slot, a StatusPill-adjacent message, and an optional Button — no interaction logic of its own. */
export function EmptyState({
  title,
  description,
  icon,
  action,
  className,
  ...rest
}: EmptyStateProps) {
  return (
    <div className={cx('flex flex-col items-center text-center gap-xs p-3xl', className)} {...rest}>
      {icon && (
        <div aria-hidden="true" className="text-text-3 text-4xl">
          {icon}
        </div>
      )}
      <p className="text-sm font-ui text-text">{title}</p>
      {description && <p className="text-xs text-text-2">{description}</p>}
      {action && (
        <Button variant="secondary" size="sm" onClick={action.onClick} className="mt-4xs">
          {action.label}
        </Button>
      )}
    </div>
  )
}
