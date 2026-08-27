import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import { mockMatchMedia } from '../test/matchMedia'
import { CapabilityProvider } from './capability'
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
  ['/book/42', 'Book'],
  ['/collections', 'Collections'],
  ['/collections/abc', 'Collection'],
  ['/discover', 'Discover'],
  ['/activity', 'Activity'],
  ['/more', 'More'],
  ['/access', 'Access'],
  ['/connect', 'Connect'],
  ['/reader/9', 'Reader'],
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
    renderRoute('/book/abc-123')
    expect(await screen.findByText(/abc-123/)).toBeInTheDocument()
  })
})
