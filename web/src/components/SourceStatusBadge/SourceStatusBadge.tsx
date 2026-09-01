import { StatusPill } from '../StatusPill/StatusPill'
import { Button } from '../Button/Button'
import type { SourceHealth } from '../../data/sources'
import { cx } from '../../lib/cx'
import { getHealthDescription } from './healthDescription'

export interface SourceStatusBadgeProps {
  health: SourceHealth
  sourceId?: string
  onCheckAgain?: () => void
  isChecking?: boolean
  className?: string
}

/**
 * SourceStatusBadge (frontend-source-management.md FR-3):
 * Renders one of eleven plainly-worded health states with status pill,
 * a "Check again" action when unreachable, and an accessibility live region.
 */
export function SourceStatusBadge({
  health,
  onCheckAgain,
  isChecking = false,
  className,
}: SourceStatusBadgeProps) {
  const { text, tone } = getHealthDescription(health, isChecking)

  return (
    <div className={cx('inline-flex items-center gap-xs flex-wrap', className)} aria-live="polite">
      <StatusPill tone={tone}>{text}</StatusPill>
      {health.status === 'unreachable' && onCheckAgain && !isChecking && (
        <Button variant="ghost" onClick={onCheckAgain} className="text-xs px-2xs py-4xs h-auto">
          Check again
        </Button>
      )}
    </div>
  )
}
