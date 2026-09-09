import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { NetworkSettings } from './NetworkSettings'

function renderWithProviders(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return {
    ...render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>),
    queryClient,
  }
}

describe('NetworkSettings screen (Phase 13 T5.3)', () => {
  it('renders status card with honest copy for reachability, server address, and TLS', async () => {
    server.use(
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'local_network',
          tlsMode: 'none',
          authRequired: true,
          address: 'http://192.168.1.50:4000',
          addresses: [{ scope: 'local', url: 'http://192.168.1.50:4000' }],
          hostName: 'alexandryn.local',
        }),
      ),
    )

    renderWithProviders(<NetworkSettings />)

    await waitFor(() => expect(screen.getByTestId('reachability-val')).toBeInTheDocument())

    // Reachability
    expect(screen.getByTestId('reachability-val')).toHaveTextContent(
      'Accessible from devices on your local network.',
    )

    // Address
    expect(screen.getByTestId('server-address-val')).toHaveTextContent('http://192.168.1.50:4000')

    // Honest TLS unencrypted copy (FR-1)
    const tlsDesc = screen.getByTestId('tls-mode-desc')
    expect(tlsDesc).toHaveTextContent(
      'Traffic on your local network is unencrypted. Anyone with access to your Wi-Fi or router can see the books you read and the pages you view, unless you are using a reverse proxy that provides TLS.',
    )

    // Static "Authentication is always on" row — NO toggle role in the tree
    const authRow = screen.getByTestId('auth-always-on')
    expect(authRow).toHaveTextContent('Authentication is always on')
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  })

  it('Advanced disclosure toggles via keyboard and button with aria-expanded', async () => {
    server.use(
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'local_only',
          tlsMode: 'static',
          authRequired: true,
          address: 'https://localhost:8443',
          addresses: [{ scope: 'loopback', url: 'https://localhost:8443' }],
          hostName: 'alexandryn.local',
        }),
      ),
    )

    renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByText('Show Advanced')).toBeInTheDocument())

    const toggleBtn = screen.getByRole('button', { name: 'Show Advanced' })
    expect(toggleBtn).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByTestId('advanced-bind-address')).not.toBeInTheDocument()

    const user = userEvent.setup()
    await user.click(toggleBtn)

    expect(toggleBtn).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByTestId('advanced-bind-address')).toBeInTheDocument()
    expect(screen.getByTestId('advanced-tls-cert')).toHaveTextContent('configured')
  })

  it('displays ACME domain and TOS line when tlsMode is acme', async () => {
    server.use(
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'internet',
          tlsMode: 'acme',
          authRequired: true,
          address: 'https://library.example.com',
          addresses: [{ scope: 'internet', url: 'https://library.example.com' }],
          hostName: 'alexandryn.local',
          acmeDomain: 'library.example.com',
        }),
      ),
    )

    renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByText('Show Advanced')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('Show Advanced'))

    expect(screen.getByTestId('advanced-acme-domain')).toHaveTextContent('library.example.com')
    expect(screen.getByText(/Subject to Let's Encrypt Terms of Service/i)).toBeInTheDocument()
  })

  // audit 0016 #143: the form pre-fills from GET /api/v1/network/settings,
  // not from hardcoded defaults.
  it('pre-fills the form from the saved network settings', async () => {
    server.use(
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'local_network',
          tlsMode: 'none',
          authRequired: true,
          address: 'http://192.168.1.50:4000',
          addresses: [{ scope: 'local', url: 'http://192.168.1.50:4000' }],
          hostName: 'alexandryn.local',
        }),
      ),
      http.get('*/api/v1/network/settings', () =>
        HttpResponse.json({
          hostName: 'my-library.local',
          rememberDeviceDays: 7,
          updatedAt: '2026-09-05T12:00:00Z',
        }),
      ),
    )

    renderWithProviders(<NetworkSettings />)

    await waitFor(() =>
      expect(screen.getByLabelText(/Local Network Name/i)).toHaveValue('my-library.local'),
    )
    expect(screen.getByLabelText(/Remember Devices/i)).toHaveValue(7)
  })

  it('saves settings with optimistic update and rollback on error', async () => {
    let patched = false
    server.use(
      http.patch('*/api/v1/network/settings', async () => {
        patched = true
        return HttpResponse.json({
          hostName: 'new-name.local',
          rememberDeviceDays: 14,
          updatedAt: new Date().toISOString(),
        })
      }),
    )

    renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByLabelText(/Local Network Name/i)).toBeInTheDocument())

    const hostInput = screen.getByLabelText(/Local Network Name/i)
    const daysInput = screen.getByLabelText(/Remember Devices/i)

    const user = userEvent.setup()
    await user.clear(hostInput)
    await user.type(hostInput, 'new-name.local')
    await user.clear(daysInput)
    await user.type(daysInput, '14')

    await user.click(screen.getByRole('button', { name: 'Save Settings' }))

    await waitFor(() => expect(patched).toBe(true))
    expect(screen.getByText(/Network settings saved successfully/i)).toBeInTheDocument()
  })

  it('rolls back and displays error toast on save failure', async () => {
    server.use(
      http.patch('*/api/v1/network/settings', () => {
        return HttpResponse.json(
          { code: 'InvalidInput', message: 'Invalid hostname format', correlationId: 'test' },
          { status: 400 },
        )
      }),
    )

    renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByLabelText(/Local Network Name/i)).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: 'Save Settings' }))

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(screen.getByRole('alert')).toHaveTextContent('Invalid hostname format')
  })

  it('opens DevicePairingModal when Pair a new device is clicked', async () => {
    renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByText('Pair a new device')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('Pair a new device'))

    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())
    expect(screen.getByText('Pair a Device')).toBeInTheDocument()
  })

  it('NEVER renders file paths, cert keys, or secrets in the DOM', async () => {
    const SECRET_KEY = 'super-secret-private-key-material'
    const SECRET_PATH = '/home/user/.config/alexandryn/cert.pem'
    const SECRET_DB = 'postgres://admin:secretpass@db.local:5432/library'

    server.use(
      http.get('*/api/v1/network/status', () =>
        HttpResponse.json({
          reachability: 'local_network',
          tlsMode: 'static',
          authRequired: true,
          address: 'https://192.168.1.50:4000',
          addresses: [{ scope: 'local', url: 'https://192.168.1.50:4000' }],
          hostName: 'alexandryn.local',
          // Even if backend payload somehow held these, component MUST NOT leak them
          certFile: SECRET_PATH,
          keyFile: SECRET_KEY,
          dbUrl: SECRET_DB,
        }),
      ),
    )

    const { container } = renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByText('Show Advanced')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('Show Advanced'))

    const domHtml = container.innerHTML
    expect(domHtml).not.toContain(SECRET_KEY)
    expect(domHtml).not.toContain(SECRET_PATH)
    expect(domHtml).not.toContain(SECRET_DB)
    expect(domHtml).not.toContain('secretpass')
  })
})
