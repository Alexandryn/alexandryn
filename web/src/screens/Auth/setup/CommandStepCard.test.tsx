import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { CommandStepCard } from './CommandStepCard'

describe('CommandStepCard', () => {
  it('renders title, rationale, target badge, and command block', () => {
    render(
      <CommandStepCard
        title="Open Firewall Port"
        rationale="Allows multicast DNS packets on UDP 5353."
        target="Host Terminal"
        command="sudo ufw allow 5353/udp"
        status="pending"
        onVerify={vi.fn()}
      />,
    )

    expect(screen.getByText('Open Firewall Port')).toBeInTheDocument()
    expect(screen.getByText('Allows multicast DNS packets on UDP 5353.')).toBeInTheDocument()
    expect(screen.getByText('Host Terminal')).toBeInTheDocument()
    expect(screen.getByText('sudo ufw allow 5353/udp')).toBeInTheDocument()
    expect(screen.getByTestId('command-step-status')).toHaveTextContent('Pending')
  })

  it('copies command to clipboard on click', async () => {
    const user = userEvent.setup()
    const writeTextMock = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      configurable: true,
      writable: true,
    })

    render(
      <CommandStepCard
        title="Check Docker Status"
        rationale="Verifies services."
        target="Docker Terminal"
        command="docker compose ps"
        status="pending"
        onVerify={vi.fn()}
      />,
    )

    const copyBtn = screen.getByRole('button', { name: 'Copy command: docker compose ps' })
    await user.click(copyBtn)

    expect(writeTextMock).toHaveBeenCalledWith('docker compose ps')
    expect(screen.getByText('Copied!')).toBeInTheDocument()
  })

  it('triggers onVerify when clicking Verify Now button', async () => {
    const user = userEvent.setup()
    const onVerifyMock = vi.fn()

    render(
      <CommandStepCard
        title="Verify Services"
        rationale="Check health."
        target="Host Terminal"
        command="curl http://alexandryn.local/readyz"
        status="pending"
        onVerify={onVerifyMock}
      />,
    )

    const verifyBtn = screen.getByRole('button', { name: 'Verify Now' })
    await user.click(verifyBtn)

    expect(onVerifyMock).toHaveBeenCalledTimes(1)
  })

  it('displays failure state with error message, fallback text, and fallback action', async () => {
    const user = userEvent.setup()
    const onFallbackMock = vi.fn()

    render(
      <CommandStepCard
        title="Allow Firewall"
        rationale="mDNS port."
        target="Host Terminal"
        command="sudo ufw allow 5353/udp"
        status="failed"
        errorMessage="Port 5353 is still filtered by the network."
        fallbackText="Connect directly via your IP: http://192.168.1.50:8080"
        fallbackLabel="Use Direct IP"
        onFallback={onFallbackMock}
        onVerify={vi.fn()}
      />,
    )

    expect(screen.getByTestId('command-step-status')).toHaveTextContent('Verification Failed')
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('Port 5353 is still filtered by the network.')).toBeInTheDocument()
    expect(screen.getByText(/Connect directly via your IP/)).toBeInTheDocument()

    const fallbackBtn = screen.getByRole('button', { name: 'Use Direct IP' })
    await user.click(fallbackBtn)
    expect(onFallbackMock).toHaveBeenCalledTimes(1)
  })

  it('displays verified state with disabled button', () => {
    render(
      <CommandStepCard
        title="Ready Check"
        rationale="Check ready."
        target="Browser"
        command="echo ready"
        status="verified"
        onVerify={vi.fn()}
      />,
    )

    expect(screen.getByTestId('command-step-status')).toHaveTextContent('Verified')
    const verifyBtn = screen.getByRole('button', { name: 'Verified' })
    expect(verifyBtn).toBeDisabled()
  })
})
