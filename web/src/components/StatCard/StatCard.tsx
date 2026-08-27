import type { HTMLAttributes, ReactNode } from 'react'
import { cx } from '../../lib/cx'

export interface StatCardProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  label: string
  value: ReactNode
  hint?: string
}

/** Label always precedes value in DOM order — a screen reader reads them as one unit, no extra ARIA needed. */
export function StatCard({ label, value, hint, className, ...rest }: StatCardProps) {
  return (
    <div
      className={cx(
        'rounded-lg border border-border bg-surface p-lg flex flex-col gap-4xs',
        className,
      )}
      {...rest}
    >
      <span className="text-xs text-text-2 font-ui">{label}</span>
      <span className="text-4xl font-ui text-text tracking-1">{value}</span>
      {hint && <span className="text-xs text-text-3">{hint}</span>}
    </div>
  )
}
