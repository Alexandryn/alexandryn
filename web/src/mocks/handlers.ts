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
  http.delete(
    '*/api/v1/collections/:id/works/:workId',
    () => new HttpResponse(null, { status: 204 }),
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

  http.get('*/api/v1/sources', () => HttpResponse.json(generatedFixtures.listSources['200'])),
  http.post('*/api/v1/sources', () =>
    HttpResponse.json(generatedFixtures.createSource['201'], { status: 201 }),
  ),
  http.get('*/api/v1/sources/:id', ({ params }) => {
    if (String(params.id).includes('non-existent') || String(params.id).includes('not-found')) {
      return HttpResponse.json(generatedFixtures.getSource['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.getSource['200'])
  }),
  http.patch('*/api/v1/sources/:id', ({ params }) => {
    if (String(params.id).includes('non-existent') || String(params.id).includes('not-found')) {
      return HttpResponse.json(generatedFixtures.updateSource['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.updateSource['200'])
  }),
  http.delete('*/api/v1/sources/:id', ({ params }) => {
    if (String(params.id).includes('non-existent') || String(params.id).includes('not-found')) {
      return HttpResponse.json(generatedFixtures.deleteSource['404'], { status: 404 })
    }
    return new HttpResponse(null, { status: 204 })
  }),
  http.post('*/api/v1/sources/:id/health-check', ({ params }) => {
    if (String(params.id).includes('non-existent') || String(params.id).includes('not-found')) {
      return HttpResponse.json(generatedFixtures.checkSourceHealth['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.checkSourceHealth['200'])
  }),
  http.get('*/api/v1/sources/:id/browse', ({ params }) => {
    if (String(params.id).includes('non-existent') || String(params.id).includes('not-found')) {
      return HttpResponse.json(generatedFixtures.browseSource['404'], { status: 404 })
    }
    if (String(params.id).includes('unavailable')) {
      return HttpResponse.json(generatedFixtures.browseSource['503'], { status: 503 })
    }
    return HttpResponse.json(generatedFixtures.browseSource['200'])
  }),
  http.get('*/api/v1/sources/:id/search', ({ params, request }) => {
    const url = new URL(request.url)
    const q = url.searchParams.get('q')
    if (String(params.id).includes('non-existent') || String(params.id).includes('not-found')) {
      return HttpResponse.json(generatedFixtures.searchSource['404'], { status: 404 })
    }
    if (String(params.id).includes('no-search')) {
      return HttpResponse.json(generatedFixtures.searchSource['409'], { status: 409 })
    }
    if (!q || q.trim() === '') {
      return HttpResponse.json(generatedFixtures.searchSource['400'], { status: 400 })
    }
    return HttpResponse.json(generatedFixtures.searchSource['200'])
  }),

  http.post('*/api/v1/import/discover', () =>
    HttpResponse.json(generatedFixtures.discoverImport['202'], { status: 202 }),
  ),
  http.get('*/api/v1/import/candidates', () =>
    HttpResponse.json(generatedFixtures.listImportCandidates['200']),
  ),
  http.post('*/api/v1/import/candidates/:id/confirm', () =>
    HttpResponse.json(generatedFixtures.confirmImportCandidate['200']),
  ),
  http.post('*/api/v1/import/candidates/:id/reject', () =>
    HttpResponse.json(generatedFixtures.rejectImportCandidate['200']),
  ),

  // Reader — the content endpoint and reading API pass through to the real
  // backend during dev; these stubs keep the frontend suite self-contained
  // where a test does not install its own reader handlers.
  http.get('*/api/v1/reading/works/:workId/progress', () =>
    HttpResponse.json({ progress: null }),
  ),
  http.post('*/api/v1/reading/works/:workId/progress', () =>
    HttpResponse.json({ progress: generatedFixtures.reportReadingProgress['200'].progress, outcome: 'advanced' }),
  ),
  http.get('*/api/v1/reading/preferences', () =>
    HttpResponse.json(generatedFixtures.getReadingPreferences['200']),
  ),
  http.put('*/api/v1/reading/preferences', async ({ request }) =>
    HttpResponse.json({ preferences: await request.json() }),
  ),
  http.get('*/api/v1/reading/editions/:editionId/bookmarks', () =>
    HttpResponse.json({ bookmarks: [] }),
  ),
  http.get('*/api/v1/reading/editions/:editionId/highlights', () =>
    HttpResponse.json({ highlights: [] }),
  ),
  http.get('*/api/v1/reading/export', () =>
    HttpResponse.json(generatedFixtures.exportReadingData['200']),
  ),

  http.get('*/api/v1/auth/setup/status', () =>
    HttpResponse.json({ isSetup: true }),
  ),
  http.post('*/api/v1/auth/login', () =>
    HttpResponse.json(generatedFixtures.login['200']),
  ),
  http.get('*/api/v1/libraries', () =>
    HttpResponse.json({
      libraries: [
        {
          id: '00000000-0000-0000-0000-000000000001',
          name: 'Default Library',
          description: 'Main library',
          allowReaderUploads: false,
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ],
    }),
  ),

  // Network & Pairing (Phase 13)
  http.post('*/api/v1/network/pair/initiate', () => {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.removeItem('alexandryn_mock_revoked')
    }
    // expiresAt is computed relative to request time, not taken verbatim
    // from the generated fixture's static example timestamp — a fixed
    // absolute date is a time bomb that silently fails every test relying
    // on a non-expired code once wall-clock time passes it.
    return HttpResponse.json(
      {
        ...generatedFixtures.initiatePairing['201'],
        expiresAt: new Date(Date.now() + 5 * 60 * 1000).toISOString(),
      },
      { status: 201 },
    )
  }),
  http.post('*/api/v1/network/pair/verify', async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as { code?: string }
    const code = body.code?.replace(/-/g, '').toUpperCase()
    const isRevoked =
      typeof window !== 'undefined' &&
      window.localStorage &&
      window.localStorage.getItem('alexandryn_mock_revoked') === 'true'
    if (
      !code ||
      code === '00000000' ||
      code === 'EXPDCODE' ||
      code === 'EXPIRED' ||
      isRevoked
    ) {
      return HttpResponse.json(generatedFixtures.verifyPairing['404'], { status: 404 })
    }
    return HttpResponse.json(generatedFixtures.verifyPairing['200'])
  }),
  http.get('*/api/v1/network/pair/:id/qr', () =>
    HttpResponse.json(generatedFixtures.getPairingQR['200']),
  ),
  http.get('*/api/v1/network/status', () =>
    HttpResponse.json(generatedFixtures.getNetworkStatus['200']),
  ),
  http.patch('*/api/v1/network/settings', async ({ request }) => {
    const body = (await request.json().catch(() => ({}))) as Record<string, unknown>
    return HttpResponse.json({
      ...generatedFixtures.updateNetworkSettings['200'],
      ...body,
      updatedAt: new Date().toISOString(),
    })
  }),
  http.delete('*/api/v1/network/pair/:id', () => {
    if (typeof window !== 'undefined' && window.localStorage) {
      window.localStorage.setItem('alexandryn_mock_revoked', 'true')
    }
    return new HttpResponse(null, { status: 204 })
  }),

  http.all('*/api/v1/*', () => HttpResponse.json(notFoundError, { status: 404 })),
]
