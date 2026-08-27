import { delay, http, HttpResponse } from 'msw'
import { generatedFixtures } from './fixtures/generated'
import { bootstrapFixture } from './fixtures/handwritten/bootstrap'
import { notFoundError } from './fixtures/handwritten/errors'

// The mock backend (frontend-shell-and-routing.md FR-6). Health handlers
// serve the contract-generated fixtures (tier a); /api/bootstrap and the
// /api/v1 catch-all are hand-written (tier b). Later tasks add an error
// path (T8) and /api/v1/library (T9).
export const handlers = [
  http.get('*/healthz', () => HttpResponse.json(generatedFixtures.getHealthz['200'])),
  http.get('*/readyz', () => HttpResponse.json(generatedFixtures.getReadyz['200'])),

  // Never resolves synchronously (FR-4): the capability value is always
  // awaited so the loading code path is real and tested, even against a mock.
  http.get('*/api/bootstrap', async () => {
    await delay(50)
    return HttpResponse.json(bootstrapFixture)
  }),

  http.all('*/api/v1/*', () => HttpResponse.json(notFoundError, { status: 404 })),
]
