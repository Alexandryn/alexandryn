import { isRouteErrorResponse, Link, useRouteError } from 'react-router-dom'
import { ErrorState } from '../components/ErrorState'
import { ApiError } from '../data/http'

/**
 * The errorElement for the shell's routed content (frontend-shell-and-
 * routing.md FR-5): a render error or a thrown API error surfaces here,
 * inside the shell, instead of a blank page or React's error overlay. An
 * ApiError still shows its correlation ID (FR-7).
 */
export function RouteError() {
  const error = useRouteError()

  const description = (() => {
    if (error instanceof ApiError) return error.message
    if (isRouteErrorResponse(error)) return `${error.status} ${error.statusText}`
    return 'The page hit an error while rendering. Reload to try again.'
  })()

  return (
    <div className="p-3xl">
      <ErrorState
        title="This screen didn't load"
        description={description}
        code={error instanceof ApiError ? error.code : undefined}
        correlationId={error instanceof ApiError ? error.correlationId : undefined}
      />
      <p className="mt-lg text-center text-lg">
        <Link to="/library" className="text-accent hover:underline">
          Go to your library
        </Link>
      </p>
    </div>
  )
}
