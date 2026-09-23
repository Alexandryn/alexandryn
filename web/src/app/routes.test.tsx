import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { server } from '../mocks/node'
import { ApiError } from '../data/http'
import { mockMatchMedia } from '../test/matchMedia'
import { CapabilityProvider } from './capability'
import { RouteError } from './RouteError'
import { AppShell } from './shell/AppShell'
import { routes } from './routes'

let media: ReturnType<typeof mockMatchMedia> | undefined
beforeEach(() => {
  localStorage.setItem('alexandryn_access_token', 'mock-access-token')
})
afterEach(() => {
  media?.restore()
  localStorage.clear()
})

function renderRoute(path: string) {
  media = mockMatchMedia(true)
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const router = createMemoryRouter(routes, { initialEntries: [path] })
  return render(
    <QueryClientProvider client={client}>
      <CapabilityProvider>
        <RouterProvider router={router} />
      </CapabilityProvider>
    </QueryClientProvider>,
  )
}

const SHARED: [string, string][] = [
  ['/library', 'Library'],
  ['/collections', 'Collections'],
  ['/discover', 'Discover'],
  ['/activity', 'Activity'],
  ['/more', 'More'],
  ['/access', 'Access'],
  ['/connect', 'Connect'],
  ['/reader/9', 'Reader'],
]

const HOST_ONLY: [string, string][] = [
  ['/sources', 'Sources'],
  ['/import', 'Import review'],
  ['/settings', 'Settings'],
  ['/system', 'System'],
  ['/settings/network', 'Network access'],
]

describe('route table', () => {
  it.each(SHARED)('%s resolves to the "%s" screen', async (path, heading) => {
    renderRoute(path)
    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
  })

  it(
    '/read/:workId/:editionId resolves to the Reader screen',
    async () => {
      renderRoute('/read/book-1/ed-1')
      // The real reader mounts here (no content mock in this suite, so it
      // settles on its error state) rather than a placeholder. Reader is
      // lazy-loaded (lazyScreens.ts) with the heaviest dependency graph
      // of any route here (EPUB parsing) — on a CPU-constrained runner,
      // Vitest's worker pool transforming this chunk's modules for the
      // first time can occasionally take longer than the suite's default
      // 5s test budget, well before "Opening book" (rendered
      // synchronously once the module loads) ever gets a chance to
      // appear — not a slow assertion, a slow import. Generous timeout
      // here only; the default stays put for every other test.
      expect(
        await screen.findByText(/opening book|could not be opened/i, {}, { timeout: 20000 }),
      ).toBeInTheDocument()
    },
    25000,
  )

  it('/book/:id resolves to WorkDetail screen', async () => {
    server.use(
      http.get('*/api/v1/works/:id', () =>
        HttpResponse.json({
          id: '42',
          title: 'The Hitchhiker Guide',
          subtitle: '',
          authors: ['Douglas Adams'],
          subjects: [],
          ownedEditions: [],
          collections: [],
        }),
      ),
    )
    renderRoute('/book/42')
    expect(
      await screen.findByRole('heading', { name: 'The Hitchhiker Guide', level: 1 }),
    ).toBeInTheDocument()
  })

  it('/collections/:id resolves to CollectionDetail screen', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', () =>
        HttpResponse.json({
          id: 'coll-1',
          name: 'Favorites',
          works: [],
        }),
      ),
    )
    renderRoute('/collections/coll-1')
    expect(await screen.findByRole('heading', { name: 'Favorites', level: 1 })).toBeInTheDocument()
  })

  it('/collection/:id resolves to CollectionDetail screen', async () => {
    server.use(
      http.get('*/api/v1/collections/:id', () =>
        HttpResponse.json({
          id: 'coll-2',
          name: 'Classics',
          works: [],
        }),
      ),
    )
    renderRoute('/collection/coll-2')
    expect(await screen.findByRole('heading', { name: 'Classics', level: 1 })).toBeInTheDocument()
  })

  it('/discover/works/:openLibraryId resolves to DiscoverWorkDetail screen', async () => {
    server.use(
      http.get('*/api/v1/discover/works/:openLibraryId', () =>
        HttpResponse.json({
          work: {
            title: 'Middlemarch',
            authors: [{ name: 'George Eliot' }],
          },
          editions: [],
        }),
      ),
    )
    renderRoute('/discover/works/OL82563W')
    expect(
      await screen.findByRole('heading', { name: 'Middlemarch', level: 1 }),
    ).toBeInTheDocument()
  })

  it('/sources/:id resolves to SourceDetail screen', async () => {
    server.use(
      http.get('*/api/v1/sources/:id', () =>
        HttpResponse.json({
          id: 'src-1',
          label: 'Personal OPDS',
          kind: 'opds',
          config: { baseUrl: 'https://opds.example.org' },
          hasCredential: false,
          health: { status: 'reachable', checkedAt: null, detail: null },
          capabilities: { canList: true, canSearch: false, canDownload: true },
        }),
      ),
    )
    renderRoute('/sources/src-1')
    expect(
      await screen.findByRole('heading', { name: 'Personal OPDS', level: 1 }),
    ).toBeInTheDocument()
  })

  it('/ redirects to /library', async () => {
    renderRoute('/')
    expect(await screen.findByRole('heading', { name: 'Library' })).toBeInTheDocument()
  })

  it('an unregistered URL resolves to NotFound, still inside the shell', async () => {
    renderRoute('/no/such/page')
    expect(
      await screen.findByRole('heading', { name: "This page doesn't exist" }),
    ).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument()
  })

  it.each(HOST_ONLY)(
    '%s is host-only: the loading state shows first, never the "%s" content optimistically',
    async (path, heading) => {
      renderRoute(path)
      expect(screen.getByRole('status')).toBeInTheDocument()
      expect(screen.queryByRole('heading', { name: heading })).not.toBeInTheDocument()
      expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
    },
  )

  it('a :id route exposes its param to the view', async () => {
    renderRoute('/reader/abc-123')
    expect(await screen.findByText(/abc-123/)).toBeInTheDocument()
  })
})

describe('RouteError (errorElement)', () => {
  function Boom(): never {
    throw new ApiError(503, {
      code: 'unavailable',
      message: 'The server is restarting.',
      correlationId: 'corr-route-999',
    })
  }

  it('renders an ApiError with its correlation ID, inside the shell, with a way out', async () => {
    media = mockMatchMedia(true)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    // The real shape: errorElement on a pathless child of the AppShell route.
    const router = createMemoryRouter(
      [
        {
          path: '/',
          element: <AppShell />,
          children: [
            { errorElement: <RouteError />, children: [{ path: 'boom', element: <Boom /> }] },
          ],
        },
      ],
      { initialEntries: ['/boom'] },
    )
    render(
      <QueryClientProvider client={client}>
        <CapabilityProvider>
          <RouterProvider router={router} />
        </CapabilityProvider>
      </QueryClientProvider>,
    )

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('correlation-id')).toHaveTextContent('corr-route-999')
    // The shell frame survived: nav still reachable, plus an explicit way back.
    expect(screen.getByRole('navigation', { name: 'Primary' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Go to your library' })).toHaveAttribute(
      'href',
      '/library',
    )
  })

  // A public route that throws is caught by the root
  // errorElement, not React Router's raw overlay. The real route tree
  // mirrors this shape: public routes as siblings of the shell under a
  // root route that carries errorElement.
  it('catches an error thrown by a public route', async () => {
    media = mockMatchMedia(true)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const router = createMemoryRouter(
      [
        {
          errorElement: <RouteError />,
          children: [{ path: '/login', element: <Boom /> }],
        },
      ],
      { initialEntries: ['/login'] },
    )
    render(
      <QueryClientProvider client={client}>
        <CapabilityProvider>
          <RouterProvider router={router} />
        </CapabilityProvider>
      </QueryClientProvider>,
    )

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Go to your library' })).toBeInTheDocument()
  })

  // Guard: the production route tree actually has a top-level errorElement
  // (not only the nested shell one), so nothing is left uncovered.
  it('the route tree has a top-level errorElement', () => {
    expect(routes[0]?.errorElement).toBeDefined()
  })
})
