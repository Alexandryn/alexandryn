import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { expectNoAxeViolations, runAxe } from '../../test/axe'
import { NetworkSettings } from '../Settings/NetworkSettings'
import { DevicePairingModal } from './DevicePairingModal'
import { ConnectScreen } from './ConnectScreen'
import { AccessScreen } from './AccessScreen'

function renderWithProviders(ui: ReactNode) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('Phase 13 Network screens accessibility (T5.7)', () => {
  it('NetworkSettings has zero axe violations', async () => {
    const { container } = renderWithProviders(<NetworkSettings />)
    await waitFor(() => expect(screen.getByTestId('reachability-val')).toBeInTheDocument())

    const violations = await runAxe(container)
    expectNoAxeViolations(violations)
  })

  it('DevicePairingModal (open) has zero axe violations', async () => {
    renderWithProviders(<DevicePairingModal open={true} onOpenChange={() => {}} />)
    await waitFor(() => expect(screen.getByTestId('pairing-code')).toBeInTheDocument())

    const dialog = screen.getByRole('dialog')
    const violations = await runAxe(dialog)
    expectNoAxeViolations(violations)
  })

  it('ConnectScreen has zero axe violations', async () => {
    const { container } = renderWithProviders(<ConnectScreen />)
    await waitFor(() => expect(screen.getByLabelText(/Pairing Code/i)).toBeInTheDocument())

    const violations = await runAxe(container)
    expectNoAxeViolations(violations)
  })

  it('AccessScreen has zero axe violations', async () => {
    const { container } = renderWithProviders(<AccessScreen />)
    await waitFor(() => expect(screen.getByTestId('access-reachability')).toBeInTheDocument())

    const violations = await runAxe(container)
    expectNoAxeViolations(violations)
  })
})
