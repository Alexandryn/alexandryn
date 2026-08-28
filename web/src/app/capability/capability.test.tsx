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
  // optimistic `granted` (audit 0004, F26 / T5 — the load-bearing
  // security property of architecture-frontend.md FR-3). `data ===
  // undefined` covers both the initial load and a rejected request.
  it('stays loading when the bootstrap fetch fails — never optimistic granted', async () => {
    server.use(http.get('*/api/bootstrap', () => HttpResponse.error()))

    wrap(
      <RequireCapability capability="settings">
        <h1>Settings</h1>
      </RequireCapability>,
    )

    for (let i = 0; i < 8; i++) {
      await Promise.resolve()
    }
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument())
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
})
