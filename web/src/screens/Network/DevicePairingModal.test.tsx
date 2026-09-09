import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'

import { server } from '../../mocks/node'
import { DevicePairingModal } from './DevicePairingModal'

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

describe('DevicePairingModal (Phase 13 T5.4)', () => {
  it('initiates pairing on open and displays QR code and code in mono', async () => {
    renderWithProviders(<DevicePairingModal open={true} onOpenChange={vi.fn()} />)

    await waitFor(() => expect(screen.getByTestId('pairing-code')).toBeInTheDocument())
    const codeEl = screen.getByTestId('pairing-code')
    expect(codeEl).toHaveTextContent('ABCD-EFGH')
    expect(codeEl).toHaveClass('font-mono')

    // QR code SVG
    const svg = screen.getByRole('img', { name: 'Device pairing QR code' })
    expect(svg).toBeInTheDocument()
  })

  it('Done button closes without revoking', async () => {
    const onOpenChange = vi.fn()
    const deleteSpy = vi.fn()

    server.use(
      http.delete('*/api/v1/network/pair/:id', () => {
        deleteSpy()
        return new HttpResponse(null, { status: 204 })
      }),
    )

    renderWithProviders(<DevicePairingModal open={true} onOpenChange={onOpenChange} />)
    await waitFor(() => expect(screen.getByText('Done')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('Done'))

    expect(onOpenChange).toHaveBeenCalledWith(false)
    expect(deleteSpy).not.toHaveBeenCalled()
  })

  it('Revoke button deletes pairing and closes', async () => {
    const onOpenChange = vi.fn()
    let deletedId: string | null = null

    server.use(
      http.delete('*/api/v1/network/pair/:id', ({ params }) => {
        deletedId = String(params.id)
        return new HttpResponse(null, { status: 204 })
      }),
    )

    renderWithProviders(<DevicePairingModal open={true} onOpenChange={onOpenChange} />)
    await waitFor(() => expect(screen.getByText('Revoke')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('Revoke'))

    await waitFor(() => expect(deletedId).toBe('00000000-0000-0000-0000-000000000001'))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('Escape key deletes pairing and closes', async () => {
    const onOpenChange = vi.fn()
    let deleted = false

    server.use(
      http.delete('*/api/v1/network/pair/:id', () => {
        deleted = true
        return new HttpResponse(null, { status: 204 })
      }),
    )

    renderWithProviders(<DevicePairingModal open={true} onOpenChange={onOpenChange} />)
    await waitFor(() => expect(screen.getByTestId('pairing-code')).toBeInTheDocument())

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape', code: 'Escape' })

    await waitFor(() => expect(deleted).toBe(true))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  // audit 0016 #142: a failed revoke used to close the modal anyway,
  // leaving a live pairing session with no feedback.
  it('keeps the modal open and shows an error when revoke fails', async () => {
    const onOpenChange = vi.fn()
    server.use(
      http.delete('*/api/v1/network/pair/:id', () =>
        HttpResponse.json({ code: 'internal', message: 'boom' }, { status: 500 }),
      ),
    )

    renderWithProviders(<DevicePairingModal open={true} onOpenChange={onOpenChange} />)
    await waitFor(() => expect(screen.getByText('Revoke')).toBeInTheDocument())

    const user = userEvent.setup()
    await user.click(screen.getByText('Revoke'))

    expect(await screen.findByRole('alert')).toHaveTextContent('may still be active')
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(screen.getByRole('button', { name: 'Retry revoke' })).toBeInTheDocument()
  })

  it('displays "This code expired" and "Generate a new code" when session is already expired', async () => {
    server.use(
      http.post('*/api/v1/network/pair/initiate', () => {
        return HttpResponse.json(
          {
            pairingId: 'expired-1',
            code: 'EXPR-CODE',
            payload: 'http://localhost/connect?c=EXPR-CODE',
            address: 'localhost',
            expiresAt: new Date(Date.now() - 1000).toISOString(),
          },
          { status: 201 },
        )
      }),
    )

    renderWithProviders(<DevicePairingModal open={true} onOpenChange={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('This code expired')).toBeInTheDocument())
    expect(screen.getByText('Generate a new code')).toBeInTheDocument()
  })
})
