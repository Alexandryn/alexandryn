import { isRouteErrorResponse, useRouteError } from 'react-router-dom'
import { ErrorState } from '../components/ErrorState'
import { ApiError } from '../data/http'

/**
 * The layout route's errorElement (frontend-shell-and-routing.md FR-5):
 * a render error or a thrown API error surfaces here instead of a blank
 * page or React's error overlay. An ApiError still shows its correlation
 * ID (FR-7).
 */
export function RouteError() {
  const error = useRouteError()

  if (error instanceof ApiError) {
    return (
      <ErrorState
        title="Something went wrong"
        description={error.message}
        code={error.code}
        correlationId={error.correlationId}
      />
    )
  }

  if (isRouteErrorResponse(error)) {
    return (
      <ErrorState
        title="Something went wrong"
        description={`${error.status} ${error.statusText}`}
      />
    )
  }

  return (
    <ErrorState
      title="Something went wrong"
      description="An unexpected error occurred. Reloading the page may help."
    />
  )
}
