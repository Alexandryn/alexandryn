import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as auth from '../../data/auth'
import * as network from '../../data/network'
import { SetupScreen } from './SetupScreen'
import type { SetupStep } from './setup/useSetupProgress'

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom')
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  }
})

function renderSetup(initialStep?: SetupStep, isSetup = false) {
  vi.spyOn(auth, 'fetchSetupStatus').mockResolvedValue({ isSetup })
  vi.spyOn(network, 'fetchNetworkStatus').mockResolvedValue({
    reachability: 'lan',
    tlsMode: 'none',
    authRequired: true,
    address: 'http://192.168.1.50:8080',
    hostName: 'alexandryn.local',
  })

  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  })

  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter>
        <SetupScreen initialStep={initialStep} />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('SetupScreen Wizard', () => {
  beforeEach(() => {
    localStorage.clear()
    mockNavigate.mockClear()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    localStorage.clear()
  })

  it('renders welcome step first and progresses to admin step', async () => {
    const user = userEvent.setup()
    renderSetup('welcome')

    expect(screen.getByTestId('step-welcome')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Welcome to Alexandryn' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Begin Installation' }))

    expect(screen.getByTestId('step-admin')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Administrator Account' })).toBeInTheDocument()
  })

  it('focuses and marks confirmPassword on password mismatch', async () => {
    const user = userEvent.setup()
    renderSetup('admin')

    await user.type(screen.getByLabelText('Admin Username'), 'admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText(/^Password/), 'password123')
    await user.type(screen.getByLabelText('Confirm Password'), 'password456')

    await user.click(screen.getByRole('button', { name: 'Initialize Alexandryn' }))

    const confirmInput = screen.getByLabelText('Confirm Password')
    expect(confirmInput).toHaveFocus()
    expect(confirmInput).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveTextContent('Passwords do not match')
  })

  it('focuses and marks password on short password', async () => {
    const user = userEvent.setup()
    renderSetup('admin')

    await user.type(screen.getByLabelText('Admin Username'), 'admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText(/^Password/), 'short')
    await user.type(screen.getByLabelText('Confirm Password'), 'short')

    await user.click(screen.getByRole('button', { name: 'Initialize Alexandryn' }))

    const passwordInput = screen.getByLabelText(/^Password/)
    expect(passwordInput).toHaveFocus()
    expect(passwordInput).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Password must be at least 8 characters long',
    )
  })

  it('submits setup admin successfully and advances to network selection', async () => {
    const setupAdminSpy = vi.spyOn(auth, 'setupAdmin').mockResolvedValue({})
    const user = userEvent.setup()
    renderSetup('admin')

    await user.type(screen.getByLabelText('Admin Username'), 'admin')
    await user.type(screen.getByLabelText('Email Address'), 'admin@example.com')
    await user.type(screen.getByLabelText(/^Password/), 'password123')
    await user.type(screen.getByLabelText('Confirm Password'), 'password123')

    await user.click(screen.getByRole('button', { name: 'Initialize Alexandryn' }))

    expect(setupAdminSpy).toHaveBeenCalledWith({
      username: 'admin',
      email: 'admin@example.com',
      password: 'password123',
    })

    await waitFor(() => {
      expect(screen.getByTestId('step-network')).toBeInTheDocument()
    })
  })

  it('selects LAN mode by default and saves network settings', async () => {
    const updateSpy = vi.spyOn(network, 'updateNetworkSettings').mockResolvedValue({
      hostName: 'alexandryn.local',
      rememberDeviceDays: 30,
      updatedAt: '2026-09-28T00:00:00Z',
    })
    const user = userEvent.setup()
    renderSetup('network')

    expect(screen.getByLabelText('Local Network (LAN)')).toBeChecked()
    expect(screen.getByLabelText('Local Hostname (.local)')).toHaveValue('alexandryn.local')

    await user.click(screen.getByRole('button', { name: 'Continue to Service Checks' }))

    expect(updateSpy).toHaveBeenCalledWith({ hostName: 'alexandryn.local' })
    expect(screen.getByTestId('step-services')).toBeInTheDocument()
  })

  it('navigates from services to command steps with copy and active verification', async () => {
    const user = userEvent.setup()
    // Mock navigator.clipboard
    const writeTextMock = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: writeTextMock },
      configurable: true,
      writable: true,
    })

    renderSetup('services')
    expect(screen.getByTestId('step-services')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'View Network Commands' }))
    expect(screen.getByTestId('step-commands')).toBeInTheDocument()

    // Find mDNS command copy button
    const copyBtn = screen.getByRole('button', {
      name: 'Copy command: sudo ufw allow 5353/udp',
    })
    await user.click(copyBtn)
    expect(writeTextMock).toHaveBeenCalledWith('sudo ufw allow 5353/udp')

    // Active verification
    const verifyButtons = screen.getAllByRole('button', { name: 'Verify Now' })
    expect(verifyButtons.length).toBeGreaterThan(0)
    await user.click(verifyButtons[0]!)

    await waitFor(() => {
      expect(screen.getAllByText('Verified').length).toBeGreaterThan(0)
    })
  })

  it('resumes from localStorage state on reload', async () => {
    localStorage.setItem(
      'alexandryn_setup_progress',
      JSON.stringify({
        step: 'network',
        networkMode: 'lan',
        hostName: 'my-library.local',
        completedSteps: ['welcome', 'admin'],
      }),
    )

    renderSetup()

    expect(screen.getByTestId('step-network')).toBeInTheDocument()
    expect(screen.getByLabelText('Local Hostname (.local)')).toHaveValue('my-library.local')
  })

  it('skips account creation if backend already reports isSetup: true', async () => {
    renderSetup('admin', true)

    await waitFor(() => {
      expect(screen.getByTestId('step-network')).toBeInTheDocument()
    })
  })

  it('completes the full flow and navigates to /library', async () => {
    const user = userEvent.setup()
    renderSetup('verification')

    expect(screen.getByTestId('step-verification')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Finish Setup' }))

    expect(screen.getByTestId('step-complete')).toBeInTheDocument()
    expect(screen.getByText('Alexandryn is Ready!')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Enter Library' }))
    expect(mockNavigate).toHaveBeenCalledWith('/library', { replace: true })
    expect(localStorage.getItem('alexandryn_setup_progress')).toBeNull()
  })
})
