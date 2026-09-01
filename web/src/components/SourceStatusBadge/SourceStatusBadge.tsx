import { StatusPill, type StatusTone } from '../StatusPill/StatusPill'
import { Button } from '../Button/Button'
import type { SourceHealth } from '../../data/sources'
import { cx } from '../../lib/cx'

export interface SourceStatusBadgeProps {
  health: SourceHealth
  sourceId?: string
  onCheckAgain?: () => void
  isChecking?: boolean
  className?: string
}

const DETAIL_TEXT: Record<string, string> = {
  timeout: "Can't connect — timed out",
  'connection-refused': "Can't connect — connection refused",
  'auth-rejected': 'Wrong username or password',
  'http-4xx': 'This source returned an error',
  'http-5xx': 'This source is having a problem right now',
  'http-3xx-unsupported': "This source tried to redirect, which isn't supported",
  'unparseable-response': 'Received an unexpected response',
  'path-not-found': 'Folder not found',
  'path-not-readable': "Folder isn't readable",
}

export function getHealthDescription(
  health: SourceHealth,
  isChecking?: boolean,
): { text: string; tone: StatusTone } {
  if (isChecking) {
    return { text: 'Checking...', tone: 'neutral' }
  }
  if (health.status === 'reachable') {
    return { text: 'Reachable', tone: 'success' }
  }
  if (health.status === 'unknown') {
    return { text: 'Checking...', tone: 'neutral' }
  }
  if (health.detail && DETAIL_TEXT[health.detail]) {
    return { text: DETAIL_TEXT[health.detail]!, tone: 'error' }
  }
  return { text: 'Unreachable', tone: 'error' }
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
    <div
      className={cx('inline-flex items-center gap-xs flex-wrap', className)}
      aria-live="polite"
    >
      <StatusPill tone={tone}>{text}</StatusPill>
      {health.status === 'unreachable' && onCheckAgain && !isChecking && (
        <Button
          variant="ghost"
          onClick={onCheckAgain}
          className="text-xs px-2xs py-4xs h-auto"
        >
          Check again
        </Button>
      )}
    </div>
  )
}
