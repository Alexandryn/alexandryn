import { http, HttpResponse } from 'msw'
import { generatedFixtures } from './fixtures/generated'
import { notFoundError } from './fixtures/handwritten/errors'

// The mock backend (frontend-shell-and-routing.md FR-6). Health handlers
// serve the contract-generated fixtures (tier a); the /api/v1 catch-all
// mirrors the Go server's own NotFoundHandler shape (tier b). Later tasks
// add /api/bootstrap (T3), an error path (T8) and /api/v1/library (T9).
export const handlers = [
  http.get('*/healthz', () => HttpResponse.json(generatedFixtures.getHealthz['200'])),
  http.get('*/readyz', () => HttpResponse.json(generatedFixtures.getReadyz['200'])),

  http.all('*/api/v1/*', () => HttpResponse.json(notFoundError, { status: 404 })),
]
