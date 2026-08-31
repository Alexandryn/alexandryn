import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { server } from '../mocks/node'
import { ApiError } from '../data/http'
import { mockMatchMedia } from '../test/matchMedia'
import { CapabilityProvider } from './capability'
import { RouteError } from './RouteError'
import { AppShell } from './shell/AppShell'
import { routes } from './routes'


let media: ReturnType<typeof mockMatchMedia> | undefined
afterEach(() => media?.restore())

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
  ['/read/book-1/ed-1', 'Reader'],
]

const HOST_ONLY: [string, string][] = [
  ['/sources', 'Sources'],
  ['/sources/7', 'Source'],
  ['/import', 'Import'],
  ['/settings', 'Settings'],
  ['/system', 'System'],
]

describe('route table (FR-1)', () => {
  it.each(SHARED)('%s resolves to the "%s" screen', async (path, heading) => {
    renderRoute(path)
    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
  })

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
    expect(
      await screen.findByRole('heading', { name: 'Favorites', level: 1 }),
    ).toBeInTheDocument()
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
    expect(
      await screen.findByRole('heading', { name: 'Classics', level: 1 }),
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



describe('RouteError (errorElement, FR-5/FR-7)', () => {
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
})
