import { delay, http, HttpResponse } from 'msw'

import { generatedFixtures } from './fixtures/generated'
import { bootstrapFixture } from './fixtures/handwritten/bootstrap'
import { notFoundError } from './fixtures/handwritten/errors'

// The mock backend (frontend-shell-and-routing.md FR-6). Health handlers
// serve the contract-generated fixtures (tier a); /api/bootstrap and the
// /api/v1 catch-all are hand-written (tier b).
// Library, works, and collections endpoints (phase 06) pass through to the real backend.
export const handlers = [
  http.get('*/healthz', () => HttpResponse.json(generatedFixtures.getHealthz['200'])),
  http.get('*/readyz', () => HttpResponse.json(generatedFixtures.getReadyz['200'])),

  // Never resolves synchronously (FR-4): the capability value is always
  // awaited so the loading code path is real and tested, even against a mock.
  http.get('*/api/bootstrap', async () => {
    await delay(50)
    return HttpResponse.json(bootstrapFixture)
  }),

  http.get('*/api/v1/library', () => HttpResponse.json(generatedFixtures.listLibrary['200'])),
  http.get('*/api/v1/works/:id', ({ params }) => {
    if (String(params.id).includes('non-existent')) {
      return HttpResponse.json(generatedFixtures.getWork['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.getWork['200'])
  }),

  http.get('*/api/v1/collections', () =>
    HttpResponse.json(generatedFixtures.listCollections['200']),
  ),
  http.get('*/api/v1/collections/:id', ({ params }) => {
    if (String(params.id).includes('non-existent')) {
      return HttpResponse.json(generatedFixtures.getCollection['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.getCollection['200'])
  }),

  http.post('*/api/v1/collections', () =>
    HttpResponse.json(generatedFixtures.createCollection['201'], { status: 201 }),
  ),
  http.patch('*/api/v1/collections/:id', () =>
    HttpResponse.json(generatedFixtures.renameCollection['200']),
  ),
  http.delete('*/api/v1/collections/:id', () => new HttpResponse(null, { status: 204 })),
  http.post('*/api/v1/collections/:id/works', () =>
    HttpResponse.json(generatedFixtures.addWorkToCollection['200']),
  ),
  http.delete('*/api/v1/collections/:id/works/:workId', () =>
    new HttpResponse(null, { status: 204 }),
  ),

  http.get('*/api/v1/discover', ({ request }) => {
    const url = new URL(request.url)
    const q = url.searchParams.get('q')
    if (!q || q.trim() === '') {
      return HttpResponse.json(generatedFixtures.searchDiscover['400'], { status: 400 })
    }
    if (q.includes('unavailable-test')) {
      return HttpResponse.json(generatedFixtures.searchDiscover['503'], { status: 503 })
    }
    if (q.includes('empty-test')) {
      return HttpResponse.json({ items: [], total: 0, limit: 20, offset: 0 })
    }
    return HttpResponse.json(generatedFixtures.searchDiscover['200'])
  }),

  http.get('*/api/v1/discover/works/:openLibraryId', ({ params }) => {
    const id = String(params.openLibraryId)
    if (id.includes('unavailable')) {
      return HttpResponse.json(generatedFixtures.getDiscoverWork['503'], { status: 503 })
    }
    if (id.includes('not-found') || id.includes('non-existent')) {
      return HttpResponse.json(generatedFixtures.getDiscoverWork['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.getDiscoverWork['200'])
  }),

  http.get('*/api/v1/discover/covers/:coverId', () => {
    return new HttpResponse(new Uint8Array([0xff, 0xd8, 0xff, 0xe0]), {
      headers: {
        'Content-Type': 'image/jpeg',
        'Cache-Control': 'public, max-age=2592000, immutable',
      },
    })
  }),

  http.all('*/api/v1/*', () => HttpResponse.json(notFoundError, { status: 404 })),
]




