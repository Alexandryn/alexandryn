import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { AccessScreen } from './AccessScreen'

function renderAccessScreen() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/access']}>
        <Routes>
          <Route path="/access" element={<AccessScreen />} />
          <Route path="/login" element={<div data-testid="login-page">Login Screen</div>} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('AccessScreen (Phase 13 T5.6)', () => {
  beforeEach(() => {
    localStorage.setItem(
      'alexandryn_user',
      JSON.stringify({
        id: 'u-reader-1',
        username: 'alice',
        email: 'alice@example.com',
        role: 'reader',
      }),
    )
  })

  afterEach(() => {
    localStorage.clear()
  })

  it('renders reader-scoped network status and capability list for reader', async () => {
    server.use(
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'local_network',
          tlsMode: 'none',
          authRequired: true,
          address: 'http://192.168.1.50:4000',
        }),
      ),
      http.get('*/api/v1/libraries', () =>
        HttpResponse.json({
          libraries: [
            {
              id: 'lib-1',
              name: 'Fiction Collection',
              description: 'Public domain novels',
              allowReaderUploads: false,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
          ],
        }),
      ),
    )

    renderAccessScreen()

    await waitFor(() => expect(screen.getByTestId('access-reachability')).toBeInTheDocument())

    expect(screen.getByTestId('access-reachability')).toHaveTextContent(
      'Accessible from devices on your local network.',
    )
    expect(screen.getByTestId('access-address')).toHaveTextContent('http://192.168.1.50:4000')

    // Role badge
    expect(screen.getByText(/Role: reader/i)).toBeInTheDocument()

    // Fixed capability list
    expect(screen.getByText(/Browse catalog and search books/i)).toBeInTheDocument()
    expect(screen.getByText(/Read books in the web reader/i)).toBeInTheDocument()

    // Upload disabled for readers when allowReaderUploads is false
    expect(screen.getByTestId('capability-upload-disabled')).toBeInTheDocument()

    // Accessible libraries list
    expect(screen.getByText('Fiction Collection')).toBeInTheDocument()

    // No hostModes cloud card
    expect(screen.queryByText(/cloud/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/hostModes/i)).not.toBeInTheDocument()
  })

  it('shows Upload enabled when active library allows reader uploads', async () => {
    server.use(
      http.get('*/api/v1/libraries', () =>
        HttpResponse.json({
          libraries: [
            {
              id: 'lib-2',
              name: 'Community Submissions',
              description: 'Allows reader uploads',
              allowReaderUploads: true,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
          ],
        }),
      ),
    )

    renderAccessScreen()

    await waitFor(() => expect(screen.getByTestId('capability-upload')).toBeInTheDocument())
    expect(screen.getByTestId('capability-upload')).toHaveTextContent(
      'Upload new books to the active library',
    )
  })

  it('Sign out of this browser logs out and navigates to /login', async () => {
    let logoutCalled = false
    server.use(
      http.post('*/api/v1/auth/logout', () => {
        logoutCalled = true
        return HttpResponse.json({ success: true })
      }),
    )

    localStorage.setItem('alexandryn_refresh_token', 'sample-refresh-token')

    renderAccessScreen()

    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Sign out of this browser' })).toBeInTheDocument(),
    )

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Sign out of this browser' }))

    await waitFor(() => expect(screen.getByTestId('login-page')).toBeInTheDocument())
    expect(logoutCalled).toBe(true)
    expect(localStorage.getItem('alexandryn_user')).toBeNull()
  })

  // audit 0016 #154: the accessible-libraries list comes from the
  // server-scoped GET /api/v1/libraries response, not from parsing the
  // access token. A stored token with an empty (or absent) libraries
  // claim must not hide libraries the server returned.
  it('lists every library the server returns without re-filtering by token', async () => {
    localStorage.setItem(
      'alexandryn_access_token',
      // header.payload.signature — payload = {"libraries":[],"role":"reader"}
      'aaa.eyJsaWJyYXJpZXMiOltdLCJyb2xlIjoicmVhZGVyIn0.bbb',
    )
    server.use(
      http.get('*/api/v1/libraries', () =>
        HttpResponse.json({
          libraries: [
            {
              id: 'lib-a',
              name: 'History Shelf',
              description: '',
              allowReaderUploads: false,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
            {
              id: 'lib-b',
              name: 'Poetry Shelf',
              description: '',
              allowReaderUploads: false,
              createdAt: new Date().toISOString(),
              updatedAt: new Date().toISOString(),
            },
          ],
        }),
      ),
    )

    renderAccessScreen()

    expect(await screen.findByText('History Shelf')).toBeInTheDocument()
    expect(screen.getByText('Poetry Shelf')).toBeInTheDocument()
  })
})
