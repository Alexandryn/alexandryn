import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { ConnectScreen } from './ConnectScreen'

function TestDestination() {
  const location = useLocation()
  return (
    <div>
      <div data-testid="target-pathname">{location.pathname}</div>
      <div data-testid="target-search">{location.search}</div>
      <div data-testid="target-state">{JSON.stringify(location.state)}</div>
    </div>
  )
}

function renderConnectScreen(initialEntries = ['/connect']) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={initialEntries}>
        <Routes>
          <Route path="/connect" element={<ConnectScreen />} />
          <Route path="/login" element={<TestDestination />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('ConnectScreen (Phase 13 T5.5)', () => {
  it('reads ?c=<code>, formats it, and strips ?c= immediately from window history', async () => {
    // Set initial window URL with ?c=ABCD-EFGH
    window.history.pushState({}, '', '/connect?c=abcd-efgh')
    expect(window.location.search).toBe('?c=abcd-efgh')

    renderConnectScreen(['/connect?c=abcd-efgh'])

    const input = screen.getByLabelText(/Pairing Code/i) as HTMLInputElement
    expect(input.value).toBe('ABCD-EFGH')

    // ?c= must be stripped from window.location immediately per acceptance criteria
    expect(window.location.search).toBe('')
  })

  it('formats code as XXXX-XXXX auto-uppercase', async () => {
    renderConnectScreen()

    const input = screen.getByLabelText(/Pairing Code/i)
    const user = userEvent.setup()

    await user.type(input, '1234abcd')
    expect(input).toHaveValue('1234-ABCD')
  })

  it('submits code and navigates to /login with enrolmentGrant in router state (NOT in URL)', async () => {
    server.use(
      http.post('*/api/v1/network/pair/verify', () =>
        HttpResponse.json({
          enrolmentGrant: 'grant-secret-token-12345',
          address: '192.168.1.50:4000',
          hostName: 'alexandryn.local',
        }),
      ),
    )

    renderConnectScreen(['/connect'])

    const input = screen.getByLabelText(/Pairing Code/i)
    const labelInput = screen.getByLabelText(/Name this device/i)
    const submitBtn = screen.getByRole('button', { name: 'Continue' })

    const user = userEvent.setup()
    await user.type(input, '3ABC-DEFG')
    await user.type(labelInput, 'Living Room iPad')
    await user.click(submitBtn)

    await waitFor(() => expect(screen.getByTestId('target-pathname')).toHaveTextContent('/login'))

    // URL search parameters MUST be clean (no grant in URL)
    expect(screen.getByTestId('target-search')).toHaveTextContent('')

    // Router state carries the grant
    const stateText = screen.getByTestId('target-state').textContent
    expect(stateText).toContain('grant-secret-token-12345')
    expect(stateText).toContain('alexandryn.local')
  })

  it('handles 404 with generic "pairing code not recognised"', async () => {
    server.use(
      http.post('*/api/v1/network/pair/verify', () =>
        HttpResponse.json(
          { code: 'NotFound', message: 'pairing code not recognised', correlationId: 'test' },
          { status: 404 },
        ),
      ),
    )

    renderConnectScreen(['/connect'])

    const input = screen.getByLabelText(/Pairing Code/i)
    const submitBtn = screen.getByRole('button', { name: 'Continue' })

    const user = userEvent.setup()
    await user.type(input, '0000-0000')
    await user.click(submitBtn)

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(screen.getByRole('alert')).toHaveTextContent('pairing code not recognised')
  })

  it('handles 429 with "Too many attempts. Wait a minute and try again."', async () => {
    server.use(
      http.post('*/api/v1/network/pair/verify', () =>
        HttpResponse.json(
          { code: 'RateLimited', message: 'Rate limit exceeded', correlationId: 'test' },
          { status: 429 },
        ),
      ),
    )

    renderConnectScreen(['/connect'])

    const input = screen.getByLabelText(/Pairing Code/i)
    const submitBtn = screen.getByRole('button', { name: 'Continue' })

    const user = userEvent.setup()
    await user.type(input, '1111-2222')
    await user.click(submitBtn)

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Too many attempts. Wait a minute and try again.',
    )
  })
})
