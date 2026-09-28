import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { ReactNode } from 'react'
import { describe, expect, it } from 'vitest'
import { server } from '../../mocks/node'
import { useCapability } from './CapabilityContext'
import { CapabilityProvider } from './CapabilityProvider'
import { RequireCapability } from './RequireCapability'

function wrap(ui: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <CapabilityProvider>{ui}</CapabilityProvider>
    </QueryClientProvider>,
  )
}

function StatusProbe() {
  const state = useCapability()
  return <p data-testid="status">{state.status}</p>
}

describe('useCapability', () => {
  it('starts loading, then resolves to granted', async () => {
    wrap(<StatusProbe />)
    expect(screen.getByTestId('status')).toHaveTextContent('loading')
    await waitFor(() => expect(screen.getByTestId('status')).toHaveTextContent('granted'))
  })

  it('throws outside a CapabilityProvider', () => {
    expect(() => render(<StatusProbe />)).toThrow(/CapabilityProvider/)
  })

  // Fail-closed: a failed bootstrap fetch must never degrade to an
  // optimistic `granted`. It surfaces a recoverable error instead of
  // an eternal spinner, but host-only content still never renders.
  it('shows a recoverable error when the bootstrap fetch fails — never optimistic granted', async () => {
    server.use(http.get('*/api/bootstrap', () => HttpResponse.error()))

    wrap(
      <RequireCapability capability="settings">
        <h1>Settings</h1>
      </RequireCapability>,
    )

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: /try again/i })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Settings' })).not.toBeInTheDocument()
  })
})

describe('RequireCapability', () => {
  it('never renders host-only content before the capability value resolves', async () => {
    wrap(
      <RequireCapability capability="sources">
        <h1>Sources</h1>
      </RequireCapability>,
    )

    // First paint, and every microtask up to resolution: no host-only
    // content, a loading status in its place.
    for (let i = 0; i < 5; i++) {
      expect(screen.queryByRole('heading', { name: 'Sources' })).not.toBeInTheDocument()
      expect(screen.getByRole('status')).toBeInTheDocument()
      await Promise.resolve()
    }

    // It appears only once granted.
    expect(await screen.findByRole('heading', { name: 'Sources' })).toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('renders informative message and sign-in action when unauthenticated and capability is denied', async () => {
    localStorage.clear()
    server.use(
      http.get('*/api/bootstrap', () =>
        HttpResponse.json({
          capabilities: {
            sources: false,
            import: false,
            settings: false,
            system: false,
            network: false,
          },
        }),
      ),
    )

    wrap(
      <RequireCapability capability="sources">
        <h1>Sources</h1>
      </RequireCapability>,
    )

    expect(await screen.findByRole('heading', { name: 'Sign in to configure sources' })).toBeInTheDocument()
    expect(
      screen.getByText(/Sources manage the filesystem directories and OPDS catalogs/i),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Sources' })).not.toBeInTheDocument()
  })

  it('renders reader-specific message and back action when authenticated as reader', async () => {
    localStorage.setItem('alexandryn_access_token', 'test-token')
    localStorage.setItem(
      'alexandryn_user',
      JSON.stringify({ id: 'user-1', username: 'reader_user', email: 'r@test.com', role: 'reader' }),
    )
    server.use(
      http.get('*/api/bootstrap', () =>
        HttpResponse.json({
          capabilities: {
            sources: false,
            import: false,
            settings: false,
            system: false,
            network: false,
          },
        }),
      ),
    )

    wrap(
      <RequireCapability capability="sources">
        <h1>Sources</h1>
      </RequireCapability>,
    )

    expect(
      await screen.findByRole('heading', { name: 'Administrator access required for sources' }),
    ).toBeInTheDocument()
    expect(screen.getByText(/Your current account has reader permissions/i)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Back to Library' })).toBeInTheDocument()
    localStorage.clear()
  })

  it('renders upload restriction message when reader import is denied', async () => {
    localStorage.setItem('alexandryn_access_token', 'test-token')
    localStorage.setItem(
      'alexandryn_user',
      JSON.stringify({ id: 'user-1', username: 'reader_user', email: 'r@test.com', role: 'reader' }),
    )
    server.use(
      http.get('*/api/bootstrap', () =>
        HttpResponse.json({
          capabilities: {
            sources: false,
            import: false,
            settings: false,
            system: false,
            network: false,
          },
        }),
      ),
    )

    wrap(
      <RequireCapability capability="import">
        <h1>Import</h1>
      </RequireCapability>,
    )

    expect(
      await screen.findByRole('heading', { name: 'Reader uploads are disabled' }),
    ).toBeInTheDocument()
    expect(
      screen.getByText(/This library is configured to disallow reader accounts from uploading/i),
    ).toBeInTheDocument()
    localStorage.clear()
  })
})

