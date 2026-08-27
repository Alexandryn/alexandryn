import type { HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import { Button } from '../Button/Button'

export interface ErrorStateProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  /** Plain-language sentence: what failed and what to do next (constitution §11). */
  title: string
  description?: string
  /** The API error's machine code, shown small and secondary. */
  code?: string
  /**
   * The correlation ID from the error response — rendered visibly but
   * understated (frontend-shell-and-routing.md FR-7), so a person can
   * quote it when reporting the problem.
   */
  correlationId?: string
  onRetry?: () => void
}

/**
 * The error treatment for a failed data fetch or a route render error
 * (FR-2/FR-5/FR-7), composed from primitives — the message leads, the
 * technical detail follows in a muted monospace line.
 */
export function ErrorState({
  title,
  description,
  code,
  correlationId,
  onRetry,
  className,
  ...rest
}: ErrorStateProps) {
  return (
    <div
      role="alert"
      className={cx(
        'mx-auto flex max-w-[36rem] flex-col items-center gap-xs p-3xl text-center',
        className,
      )}
      {...rest}
    >
      <p className="text-3xl font-medium text-text">{title}</p>
      {description ? <p className="text-lg text-text-2">{description}</p> : null}
      {onRetry ? (
        <Button variant="secondary" size="sm" onClick={onRetry} className="mt-xs">
          Try again
        </Button>
      ) : null}
      {code || correlationId ? (
        <p className="mt-xs font-mono text-2xs text-text-3">
          {code ? <span>{code}</span> : null}
          {code && correlationId ? <span aria-hidden="true"> · </span> : null}
          {correlationId ? (
            <span>
              reference <span data-testid="correlation-id">{correlationId}</span>
            </span>
          ) : null}
        </p>
      ) : null}
    </div>
  )
}
