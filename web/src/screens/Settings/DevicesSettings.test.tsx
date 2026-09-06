import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { server } from '../../mocks/node'
import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { DevicesSettings } from './DevicesSettings'

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

const mockDevices = [
  {
    id: 'dev-1',
    label: 'MacBook Pro',
    deviceClass: 'desktop',
    enrolledVia: 'password_login',
    createdAt: '2026-09-01T10:00:00Z',
    lastSeenAt: new Date(Date.now() - 2 * 60 * 1000).toISOString(), // Active now
    lastSyncedAt: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
    revokedAt: null,
  },
  {
    id: 'dev-2',
    label: 'Pixel 8',
    deviceClass: 'phone',
    enrolledVia: 'pairing_code',
    createdAt: '2026-09-02T10:00:00Z',
    lastSeenAt: new Date(Date.now() - 15 * 60 * 1000).toISOString(), // Active 15 minutes ago
    lastSyncedAt: null,
    revokedAt: null,
  },
  {
    id: 'dev-3',
    label: 'iPad Air',
    deviceClass: 'tablet',
    enrolledVia: 'pairing_code',
    createdAt: '2026-09-03T10:00:00Z',
    lastSeenAt: new Date(Date.now() - 3 * 3600 * 1000).toISOString(), // Active 3 hours ago
    lastSyncedAt: new Date(Date.now() - 3 * 3600 * 1000).toISOString(),
    revokedAt: null,
  },
]

describe('DevicesSettings component tests (Phase 14)', () => {
  it('renders active devices and filters out revoked devices (FR-1)', async () => {
    const mixedDevices = [
      ...mockDevices,
      {
        id: 'dev-revoked',
        label: 'Old Phone',
        deviceClass: 'phone',
        enrolledVia: 'pairing_code',
        createdAt: '2026-08-01T10:00:00Z',
        lastSeenAt: '2026-08-10T10:00:00Z',
        lastSyncedAt: null,
        revokedAt: '2026-08-15T10:00:00Z',
      },
    ]

    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mixedDevices })),
    )

    renderWithProviders(<DevicesSettings />)

    await waitFor(() => expect(screen.getByText('MacBook Pro')).toBeInTheDocument())
    expect(screen.getByText('Pixel 8')).toBeInTheDocument()
    expect(screen.getByText('iPad Air')).toBeInTheDocument()

    // Revoked device must NOT be rendered
    expect(screen.queryByText('Old Phone')).not.toBeInTheDocument()
  })

  it('shows label, kind, last-seen, and last-synced (FR-2)', async () => {
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
    )

    renderWithProviders(<DevicesSettings />)

    await waitFor(() => expect(screen.getByText('MacBook Pro')).toBeInTheDocument())

    // Kind line
    expect(screen.getByText('Desktop · direct login')).toBeInTheDocument()
    expect(screen.getByText('Phone · paired by code')).toBeInTheDocument()

    // Relative times
    expect(screen.getByText('Active now')).toBeInTheDocument()
    expect(screen.getByText('Never synced')).toBeInTheDocument()
  })

  it('does NOT render IP address or session-descriptor text (FR-3)', async () => {
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
    )

    renderWithProviders(<DevicesSettings />)

    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())

    // Excluded fields from canvas: no IP address, no Owner/Remembered device/Expires in
    expect(screen.queryByText(/192\.168\./)).not.toBeInTheDocument()
    expect(screen.queryByText('Owner')).not.toBeInTheDocument()
    expect(screen.queryByText('Remembered device')).not.toBeInTheDocument()
    expect(screen.queryByText(/Expires in/)).not.toBeInTheDocument()
  })

  it('treats zero-row response as unexpected/error state rather than normal empty state (FR-9)', async () => {
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: [] })),
    )

    renderWithProviders(<DevicesSettings />)

    await waitFor(() =>
      expect(
        screen.getByText('No connected devices reported by server.'),
      ).toBeInTheDocument(),
    )
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    // No friendly empty state like "You have no devices"
    expect(screen.queryByText(/you have no devices/i)).not.toBeInTheDocument()
  })

  it('re-fetches on tab activation / mount (FR-1)', async () => {
    let fetchCount = 0
    server.use(
      http.get('*/api/v1/devices', () => {
        fetchCount++
        return HttpResponse.json({ devices: mockDevices })
      }),
    )

    const { unmount } = renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())
    expect(fetchCount).toBe(1)

    unmount()

    renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())
    expect(fetchCount).toBe(2)
  })

  it('opens confirmation dialog naming the specific device on Revoke click (FR-4, FR-6)', async () => {
    const user = userEvent.setup()
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
    )

    renderWithProviders(<DevicesSettings />)

    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())

    const revokeBtn = screen.getByRole('button', { name: 'Revoke Pixel 8' })
    await user.click(revokeBtn)

    // Confirmation dialog visible
    const dialog = screen.getByRole('dialog')
    expect(dialog).toBeInTheDocument()
    expect(within(dialog).getByText('Revoke Pixel 8?')).toBeInTheDocument()
    expect(
      within(dialog).getByText(/Are you sure you want to revoke Pixel 8\?/),
    ).toBeInTheDocument()
  })

  it('cancel in confirmation dialog closes dialog and sends no request (FR-6)', async () => {
    const user = userEvent.setup()
    let deleteCalled = false
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
      http.delete('*/api/v1/devices/:id', () => {
        deleteCalled = true
        return new HttpResponse(null, { status: 204 })
      }),
    )

    renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Revoke Pixel 8' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(deleteCalled).toBe(false)
  })

  it('closing confirmation dialog with Escape closes it without sending request (FR-6, FR-8)', async () => {
    const user = userEvent.setup()
    let deleteCalled = false
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
      http.delete('*/api/v1/devices/:id', () => {
        deleteCalled = true
        return new HttpResponse(null, { status: 204 })
      }),
    )

    renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Revoke Pixel 8' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(deleteCalled).toBe(false)
  })

  it('confirming revoke calls DELETE, removes row on 204, and announces success (FR-5, FR-7)', async () => {
    const user = userEvent.setup()
    let deletedId = ''
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
      http.delete('*/api/v1/devices/:id', ({ params }) => {
        deletedId = params.id as string
        return new HttpResponse(null, { status: 204 })
      }),
    )

    renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())

    await user.click(screen.getByRole('button', { name: 'Revoke Pixel 8' }))
    const dialog = screen.getByRole('dialog')

    // Confirm revoke
    await user.click(within(dialog).getByRole('button', { name: 'Revoke' }))

    await waitFor(() => expect(deletedId).toBe('dev-2'))

    // Live region announcement for screen readers
    const statusRegion = screen.getByRole('status')
    await waitFor(() => expect(statusRegion).toHaveTextContent('Revoked Pixel 8'))
  })

  it('handles 404 / 409 error with inline message and triggers re-fetch (FR-5)', async () => {
    const user = userEvent.setup()
    let getCount = 0
    server.use(
      http.get('*/api/v1/devices', () => {
        getCount++
        return HttpResponse.json({ devices: mockDevices })
      }),
      http.delete('*/api/v1/devices/:id', () => {
        return HttpResponse.json(
          { code: 'conflict', message: 'paired device is already revoked' },
          { status: 409 },
        )
      }),
    )

    renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('Pixel 8')).toBeInTheDocument())
    expect(getCount).toBe(1)

    await user.click(screen.getByRole('button', { name: 'Revoke Pixel 8' }))
    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Revoke' }))

    // Error alert displayed
    await waitFor(() => {
      const alert = screen.getByRole('alert')
      expect(within(alert).getByText('paired device is already revoked')).toBeInTheDocument()
    })

    // Re-fetch was triggered (getCount incremented)
    await waitFor(() => expect(getCount).toBe(2))
  })

  it('DevicesSettings has zero axe violations', async () => {
    server.use(
      http.get('*/api/v1/devices', () => HttpResponse.json({ devices: mockDevices })),
    )

    const { container } = renderWithProviders(<DevicesSettings />)
    await waitFor(() => expect(screen.getByText('MacBook Pro')).toBeInTheDocument())

    const violations = await runAxe(container)
    expectNoAxeViolations(violations)
  })
})
