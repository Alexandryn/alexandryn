import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { LoginScreen } from './LoginScreen'

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc">{loc.pathname + loc.search}</div>
}

function renderLogin(initialEntry: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <LocationProbe />
        <Routes>
          <Route path="/login" element={<LoginScreen />} />
          <Route path="/forgot-password" element={<div data-testid="forgot-page">forgot</div>} />
          <Route path="/library" element={<div data-testid="library-page">library</div>} />
          <Route path="/reader/7" element={<div data-testid="reader-page">reader</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

async function signIn() {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Email or Username'), 'librarian')
  await user.type(screen.getByLabelText('Password'), 'correct-horse')
  await user.click(screen.getByRole('button', { name: 'Sign In' }))
}

describe('LoginScreen (audit 0016 #161)', () => {
  afterEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  it('offers a Forgot your password link to /forgot-password', () => {
    renderLogin('/login')
    expect(screen.getByRole('link', { name: 'Forgot your password?' })).toHaveAttribute(
      'href',
      '/forgot-password',
    )
  })

  it('returns to the ?returnTo= destination after a successful sign-in', async () => {
    renderLogin('/login?returnTo=%2Freader%2F7')
    await signIn()
    await waitFor(() => expect(screen.getByTestId('reader-page')).toBeInTheDocument())
  })

  it('falls back to /library when no return destination is given', async () => {
    renderLogin('/login')
    await signIn()
    await waitFor(() => expect(screen.getByTestId('library-page')).toBeInTheDocument())
  })

  it('shows a confirmation after a password reset', () => {
    render(
      <QueryClientProvider
        client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
      >
        <MemoryRouter initialEntries={[{ pathname: '/login', state: { passwordReset: true } }]}>
          <Routes>
            <Route path="/login" element={<LoginScreen />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    )
    expect(screen.getByRole('status')).toHaveTextContent('Your password has been changed')
  })

  it('surfaces a 401 as an error without navigating away', async () => {
    server.use(
      http.post('*/api/v1/auth/login', () =>
        HttpResponse.json(
          { code: 'unauthorized', message: 'invalid username or password' },
          { status: 401 },
        ),
      ),
    )
    renderLogin('/login')
    await signIn()
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.getByTestId('loc')).toHaveTextContent('/login')
  })

  // audit 0016 #157: after a reload on /login the pairing enrolment grant
  // is gone from router state, but the sessionStorage mirror recovers it,
  // and it is cleared once the login completes.
  it('recovers the enrolment grant from sessionStorage after a reload', async () => {
    let sentGrant: string | undefined
    server.use(
      http.post('*/api/v1/auth/login', async ({ request }) => {
        sentGrant = ((await request.json()) as { enrolmentGrant?: string }).enrolmentGrant
        return HttpResponse.json({
          user: { id: 'u', username: 'librarian', email: 'a@b.c', role: 'admin' },
          accessToken: 'a',
          refreshToken: 'r',
        })
      }),
    )
    sessionStorage.setItem(
      'alexandryn_pending_enrolment',
      JSON.stringify({ enrolmentGrant: 'grant-xyz', hostName: 'home.local' }),
    )

    renderLogin('/login') // no router state — simulates the reload
    await signIn()

    await waitFor(() => expect(sentGrant).toBe('grant-xyz'))
    await waitFor(() => expect(screen.getByTestId('library-page')).toBeInTheDocument())
    expect(sessionStorage.getItem('alexandryn_pending_enrolment')).toBeNull()
  })
})
