import type { ReactNode } from 'react'
import type { UseQueryResult } from '@tanstack/react-query'
import { ApiError } from '../../data/http'
import { Spinner } from '../Spinner/Spinner'
import { ErrorState } from './ErrorState'

export interface QueryResultProps<T> {
  query: UseQueryResult<T>
  loadingLabel?: string
  errorTitle?: string
  children: (data: T) => ReactNode
}

/**
 * Renders the three states of a TanStack Query result — loading, error
 * (with the response's correlation ID and a retry, FR-2/FR-7), or the
 * data. Keeps every screen's fetch handling identical; the empty-result
 * case (FR-8) is the screen's own to distinguish, since "empty" is a
 * successful response.
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

  if (query.isError) {
    const err = query.error
    return (
      <ErrorState
        title={errorTitle}
        description={err instanceof ApiError ? err.message : undefined}
        code={err instanceof ApiError ? err.code : undefined}
        correlationId={err instanceof ApiError ? err.correlationId : undefined}
        onRetry={() => void query.refetch()}
      />
    )
  }

  return <>{children(query.data)}</>
}
