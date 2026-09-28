import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as auth from '../../data/auth'
import { RequireAuth } from './RequireAuth'

function renderWithAuth(initialEntry = '/protected') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route
            path="/protected"
            element={
              <RequireAuth>
                <div data-testid="protected-content">Secret Library Content</div>
              </RequireAuth>
            }
          />
          <Route path="/setup" element={<div data-testid="setup-screen">Setup Screen</div>} />
          <Route path="/login" element={<div data-testid="login-screen">Login Screen</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('RequireAuth', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    localStorage.clear()
  })

  it('shows spinner while checking setup status', () => {
    vi.spyOn(auth, 'fetchSetupStatus').mockReturnValue(new Promise(() => {}))
    renderWithAuth()
    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.getByText('Checking system status')).toBeInTheDocument()
  })

  it('redirects to /setup when system is uninitialized (isSetup: false)', async () => {
    vi.spyOn(auth, 'fetchSetupStatus').mockResolvedValue({ isSetup: false })
    renderWithAuth()
    expect(await screen.findByTestId('setup-screen')).toBeInTheDocument()
  })

  it('shows retry connection view when setupStatus query fails (prevents silent /login redirect)', async () => {
    vi.spyOn(auth, 'fetchSetupStatus').mockRejectedValue(new Error('503 Service Unavailable'))
    renderWithAuth()
    expect(await screen.findByText('Connecting to Alexandryn')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry Connection' })).toBeInTheDocument()
    expect(screen.queryByTestId('login-screen')).toBeNull()
  })

  it('redirects to /login when system is initialized but no token exists', async () => {
    vi.spyOn(auth, 'fetchSetupStatus').mockResolvedValue({ isSetup: true })
    renderWithAuth()
    expect(await screen.findByTestId('login-screen')).toBeInTheDocument()
  })

  it('renders children when system is initialized and accessToken exists', async () => {
    vi.spyOn(auth, 'fetchSetupStatus').mockResolvedValue({ isSetup: true })
    localStorage.setItem('alexandryn_access_token', 'valid-jwt-token')
    renderWithAuth()
    expect(await screen.findByTestId('protected-content')).toBeInTheDocument()
  })
})
