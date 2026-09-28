import { screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { renderWithProviders } from '../../test/renderWithProviders'
import { Sidebar } from './Sidebar'

describe('Sidebar Data Integrity and Dynamic Badges', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders zero counts and no phantom badges when data stores are empty', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json({ works: [], nextCursor: null }),
      ),
      http.get('*/api/v1/sources', () =>
        HttpResponse.json({ sources: [] }),
      ),
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({ collections: [] }),
      ),
      http.get('*/api/v1/libraries', () =>
        HttpResponse.json({
          libraries: [
            {
              id: '00000000-0000-0000-0000-000000000001',
              name: 'My Personal Library',
              description: '',
              allowReaderUploads: false,
              createdAt: '2026-01-01T00:00:00Z',
              updatedAt: '2026-01-01T00:00:00Z',
            },
          ],
        }),
      ),
      http.get('*/api/v1/devices', () =>
        HttpResponse.json({ devices: [] }),
      ),
    )

    renderWithProviders(<Sidebar />, { routerEntries: ['/library'] })

    // Active library name from API
    expect(await screen.findByText('My Personal Library')).toBeInTheDocument()

    // Real dynamic counts, not phantom "1,284 BOOKS · 4 SOURCES"
    expect(await screen.findByText('0 BOOKS · 0 SOURCES')).toBeInTheDocument()
    expect(screen.queryByText('1,284 BOOKS · 4 SOURCES')).not.toBeInTheDocument()
    expect(screen.queryByText('1284')).not.toBeInTheDocument()

    // No phantom badges
    expect(screen.queryByText('4')).not.toBeInTheDocument()
    expect(screen.queryByText('5')).not.toBeInTheDocument()

    // Real device count in hosting card, not hardcoded "2 devices connected"
    expect(screen.getByText('0 devices connected')).toBeInTheDocument()
    expect(screen.queryByText('2 devices connected')).not.toBeInTheDocument()
    expect(screen.queryByText('192.168.1.24:8474')).not.toBeInTheDocument()
  })

  it('renders dynamic counts and badges matching actual database records', async () => {
    server.use(
      http.get('*/api/v1/library', () =>
        HttpResponse.json({
          works: [
            {
              id: 'w-1',
              title: 'Book 1',
              subtitle: '',
              authors: ['Author A'],
              isOwned: true,
              collections: [],
            },
            {
              id: 'w-2',
              title: 'Book 2',
              subtitle: '',
              authors: ['Author B'],
              isOwned: true,
              collections: [],
            },
          ],
          nextCursor: null,
        }),
      ),
      http.get('*/api/v1/sources', () =>
        HttpResponse.json({
          sources: [
            {
              id: 's-1',
              label: 'Main Folder',
              kind: 'local-folder',
              config: { basePath: '/books' },
              hasCredential: false,
              health: { status: 'reachable', checkedAt: null, detail: null },
              capabilities: { canList: true, canSearch: true, canDownload: true },
            },
          ],
        }),
      ),
      http.get('*/api/v1/collections', () =>
        HttpResponse.json({
          collections: [
            { id: 'c-1', name: 'Favorites', workCount: 2 },
            { id: 'c-2', name: 'To Read', workCount: 0 },
            { id: 'c-3', name: 'Reference', workCount: 1 },
          ],
        }),
      ),
      http.get('*/api/v1/devices', () =>
        HttpResponse.json({
          devices: [
            {
              id: 'd-1',
              label: 'iPad',
              deviceClass: 'tablet',
              enrolledVia: 'qr',
              createdAt: '2026-01-01T00:00:00Z',
              lastSeenAt: '2026-01-01T00:00:00Z',
            },
          ],
        }),
      ),
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'local_network',
          tlsMode: 'none',
          authRequired: true,
          address: '10.0.0.42:8474',
        }),
      ),
    )

    renderWithProviders(<Sidebar />, { routerEntries: ['/library'] })

    // Header displays real counts: 2 books, 1 source
    expect(await screen.findByText('2 BOOKS · 1 SOURCE')).toBeInTheDocument()

    // Dynamic badges rendered for items with count > 0
    await waitFor(() => {
      expect(screen.getByText('2')).toBeInTheDocument() // 2 works for /library
      expect(screen.getByText('1')).toBeInTheDocument() // 1 source for /sources
      expect(screen.getByText('3')).toBeInTheDocument() // 3 collections for /collections
    })

    // Hosting card displays dynamic network address and device count
    expect(screen.getByText('10.0.0.42:8474')).toBeInTheDocument()
    expect(screen.getByText('1 device connected')).toBeInTheDocument()
  })
})
