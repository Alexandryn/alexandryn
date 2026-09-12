import type { ReactNode } from 'react'
import type { UseQueryResult } from '@tanstack/react-query'
import { cx } from '../../lib/cx'
import { ApiError } from '../../data/http'
import { Button } from '../Button/Button'
import { Spinner } from '../Spinner/Spinner'
import { ErrorState } from './ErrorState'

export interface QueryResultProps<T> {
  query: UseQueryResult<T>
  loadingLabel?: string
  errorTitle?: string
  children: (data: T) => ReactNode
}

function correlationIdOf(error: unknown): string | undefined {
  return error instanceof ApiError ? error.correlationId : undefined
}

/**
 * Renders the states of a TanStack Query result:
 *  - no data yet, loading          → a spinner
 *  - no data yet, failed           → a full ErrorState with the response's
 *                                    correlation ID and a retry
 *  - data present, background error → the data stays visible with a small
 *                                    non-blocking notice — stale-but-useful
 *                                    data is never blanked on a transient
 *                                    failure (this spec's Failure modes
 *                                    table / stale-while-revalidate)
 *  - data present                  → the data
 *
 * The empty-result case is the screen's own to distinguish, since
 * "empty" is a successful response.
 */
export function QueryResult<T>({
  query,
  loadingLabel = 'Loading',
  errorTitle = 'Something went wrong loading this',
  children,
}: QueryResultProps<T>) {
  if (query.isPending) {
    return <Spinner label={loadingLabel} className="m-3xl" />
  }

  if (query.data === undefined) {
    // Not pending and still no data — the fetch failed with nothing cached.
    const err = query.error
    return (
      <ErrorState
        title={errorTitle}
        description={err instanceof ApiError ? err.message : undefined}
        code={err instanceof ApiError ? err.code : undefined}
        correlationId={correlationIdOf(err)}
        onRetry={() => void query.refetch()}
      />
    )
  }

  // Data is present. A background refetch that failed leaves `status` at
  // 'success' with the error in `failureReason` (not `error`) — surface it
  // as a non-blocking notice without hiding the data.
  const backgroundError = query.fetchStatus === 'idle' ? query.failureReason : null

  return (
    <>
      {backgroundError ? (
        <StaleDataNotice
          correlationId={correlationIdOf(backgroundError)}
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {children(query.data)}
    </>
  )
}

function StaleDataNotice({
  correlationId,
  onRetry,
}: {
  correlationId?: string
  onRetry: () => void
}) {
  return (
    <div
      role="status"
      className={cx(
        'mb-lg flex flex-wrap items-center gap-md rounded-2xs',
        'border border-border bg-surface-2 px-md py-xs text-lg text-text-2',
      )}
    >
      <span>Showing the last loaded version — couldn&apos;t refresh just now.</span>
      <Button variant="ghost" size="sm" onClick={onRetry}>
        Try again
      </Button>
      {correlationId ? (
        <span className="font-mono text-2xs text-text-3">
          reference <span data-testid="correlation-id">{correlationId}</span>
        </span>
      ) : null}
    </div>
  )
}
