import type { ReactElement, ReactNode } from 'react'
import { render, type RenderResult } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'

export interface RenderWithProvidersOptions {
  /** Reuse a specific client (e.g. to inspect its cache); one is made per call otherwise. */
  queryClient?: QueryClient
  /**
   * When set, `ui` is mounted as the element of a catch-all route inside a
   * memory router seeded with these entries. Omit for a plain render.
   */
  routerEntries?: string[]
}

/**
 * Shared test render: a QueryClient with retries disabled (so an error
 * path resolves deterministically instead of backing off), optionally
 * wrapped in a memory router. Tests that need the real route tree build
 * their own `createMemoryRouter` and pass only `queryClient`.
 */
export function renderWithProviders(
  ui: ReactElement,
  { queryClient, routerEntries }: RenderWithProvidersOptions = {},
): RenderResult & { queryClient: QueryClient } {
  const client = queryClient ?? new QueryClient({ defaultOptions: { queries: { retry: false } } })

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>
  }

  const tree = routerEntries ? (
    <RouterProvider
      router={createMemoryRouter([{ path: '*', element: ui }], {
        initialEntries: routerEntries,
      })}
    />
  ) : (
    ui
  )

  return Object.assign(render(tree, { wrapper: Wrapper }), { queryClient: client })
}
