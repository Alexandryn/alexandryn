import React, { useContext, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { QueryClientContext, useQuery } from '@tanstack/react-query'
import { Button } from '../../components/Button'
import { Input } from '../../components/Input'
import { AlexAvatar } from '../../components/Mascot'
import { StatusPill } from '../../components/StatusPill/StatusPill'
import { fetchSetupStatus, setupAdmin } from '../../data/auth'
import { ApiError } from '../../data/http'
import { fetchNetworkStatus, updateNetworkSettings } from '../../data/network'
import { cx } from '../../lib/cx'
import { CommandStepCard } from './setup/CommandStepCard'
import { SETUP_STEPS, type SetupStep, useSetupProgress } from './setup/useSetupProgress'

export interface SetupScreenProps {
  initialStep?: SetupStep
}

export function SetupScreen({ initialStep }: SetupScreenProps) {
  const navigate = useNavigate()
  const queryClient = useContext(QueryClientContext)

  // Fetch initial setup status from backend
  const { data: backendStatus, refetch: refetchSetupStatus } = useQuery({
    queryKey: ['setupStatus'],
    queryFn: fetchSetupStatus,
    staleTime: 60_000,
  })

  const {
    step,
    setStep,
    networkMode,
    setNetworkMode,
    hostName,
    setHostName,
    markStepComplete,
    commandStatus,
    setCommandStatus,
    resetProgress,
  } = useSetupProgress(backendStatus?.isSetup ?? false)

  // Use initialStep if passed in (useful for testing or direct navigation)
  useEffect(() => {
    if (initialStep) {
      setStep(initialStep)
    }
  }, [initialStep, setStep])

  // Network status query (enabled after admin creation or when entering network/services)
  const { data: networkStatus, refetch: refetchNetwork } = useQuery({
    queryKey: ['network', 'status'],
    queryFn: fetchNetworkStatus,
    staleTime: 30_000,
  })

  // Admin Account form state
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [adminError, setAdminError] = useState<string | null>(null)
  const [adminLoading, setAdminLoading] = useState(false)

  const passwordRef = useRef<HTMLInputElement>(null)
  const confirmPasswordRef = useRef<HTMLInputElement>(null)

  // Verification state for command steps
  const [verifyingCommand, setVerifyingCommand] = useState<string | null>(null)
  const [copiedUrl, setCopiedUrl] = useState(false)

  // Step indices
  const currentStepIndex = SETUP_STEPS.indexOf(step)
  const isDesktop = typeof window !== 'undefined' && 'alexandryn' in window

  // Handle Admin Submission
  const handleAdminSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setAdminError(null)

    if (password !== confirmPassword) {
      setAdminError('Passwords do not match. Check both fields and try again.')
      confirmPasswordRef.current?.focus()
      return
    }
    if (password.length < 8) {
      setAdminError(
        'Password must be at least 8 characters long. Choose a longer password and try again.',
      )
      passwordRef.current?.focus()
      return
    }

    setAdminLoading(true)
    try {
      await setupAdmin({ username, email, password })
      queryClient?.clear()
      markStepComplete('admin')
      setStep('network')
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 409 || err.message === 'system already initialized') {
          // If already initialized, mark complete and proceed
          markStepComplete('admin')
          setStep('network')
          return
        }
        if (err.status === 403 || err.code === 'Forbidden' || err.message.includes('origin')) {
          setAdminError(
            'Setup request was blocked by origin validation. Connect through an allowed address or configure CORS_ALLOWED_ORIGINS.',
          )
        } else if (err.status === 400 || err.code === 'InvalidInput') {
          setAdminError(
            err.message && err.message !== 'malformed request payload'
              ? err.message
              : 'Invalid account details. Check username, email, and password requirements, then try again.',
          )
        } else {
          setAdminError(
            err.message || 'Could not initialize Alexandryn. Check your connection and try again.',
          )
        }
      } else if (err instanceof Error) {
        setAdminError(err.message)
      } else {
        setAdminError('Could not initialize Alexandryn. Check your connection and try again.')
      }
    } finally {
      setAdminLoading(false)
    }
  }

  // Handle Network Step Submission
  const handleNetworkSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (networkMode === 'lan' && hostName.trim()) {
      try {
        await updateNetworkSettings({ hostName: hostName.trim() })
      } catch {
        // Non-blocking; continue to service verification
      }
    }
    markStepComplete('network')
    setStep('services')
  }

  // Verify mDNS command
  const handleVerifyMdns = async () => {
    setVerifyingCommand('mdns')
    setCommandStatus('mdns', 'executing')
    try {
      const res = await refetchNetwork()
      if (res.data) {
        setCommandStatus('mdns', 'verified')
      } else {
        setCommandStatus('mdns', 'failed')
      }
    } catch {
      setCommandStatus('mdns', 'failed')
    } finally {
      setVerifyingCommand(null)
    }
  }

  // Verify Docker command
  const handleVerifyDocker = async () => {
    setVerifyingCommand('docker')
    setCommandStatus('docker', 'executing')
    try {
      const res = await refetchSetupStatus()
      if (res.data) {
        setCommandStatus('docker', 'verified')
      } else {
        setCommandStatus('docker', 'failed')
      }
    } catch {
      setCommandStatus('docker', 'failed')
    } finally {
      setVerifyingCommand(null)
    }
  }

  // Finish Setup & Launch Library
  const handleFinish = () => {
    resetProgress()
    queryClient?.clear()
    navigate('/library', { replace: true })
  }

  // Primary URL calculation
  const primaryUrl =
    networkMode === 'lan'
      ? `http://${hostName.trim() || 'alexandryn.local'}`
      : 'http://127.0.0.1:8080'

  const handleCopyUrl = async () => {
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard) {
        await navigator.clipboard.writeText(primaryUrl)
        setCopiedUrl(true)
        setTimeout(() => setCopiedUrl(false), 2000)
      }
    } catch {
      // Ignore
    }
  }

  const stepTitles: Record<SetupStep, string> = {
    welcome: 'Welcome',
    admin: 'Administrator Account',
    network: 'Network Selection',
    services: 'Service Verification',
    commands: 'System Commands',
    verification: 'Checklist',
    complete: 'Setup Complete',
  }

  return (
    <main className="min-h-screen flex items-center justify-center bg-background p-md font-sans">
      <div className="w-full max-w-xl bg-surface p-xl rounded-lg border border-border shadow-lg flex flex-col gap-lg">
        {/* Wizard Progress Header */}
        <div className="flex flex-col gap-xs" aria-label="Setup progress">
          <div className="flex items-center justify-between text-xs text-text-3 font-medium">
            <span>
              Step {currentStepIndex + 1} of {SETUP_STEPS.length}: {stepTitles[step]}
            </span>
            <span>{Math.round(((currentStepIndex + 1) / SETUP_STEPS.length) * 100)}%</span>
          </div>
          <div className="h-1.5 w-full bg-surface-2 rounded-full overflow-hidden">
            <div
              className="h-full bg-accent transition-all duration-300"
              style={{
                width: `${((currentStepIndex + 1) / SETUP_STEPS.length) * 100}%`,
              }}
            />
          </div>
        </div>

        {/* ── STEP 1: WELCOME ────────────────────────────────────────── */}
        {step === 'welcome' && (
          <div className="flex flex-col gap-md" data-testid="step-welcome">
            <div className="text-center">
              <AlexAvatar size="lg" className="mx-auto mb-md" />
              <h1 className="text-2xl font-serif font-bold text-text mb-xs">
                Welcome to Alexandryn
              </h1>
              <p className="text-sm text-text-3 leading-relaxed">
                Your private, self-hosted digital library for collecting, reading, and sharing
                books.
              </p>
            </div>

            <div className="rounded-lg border border-border bg-surface-2 p-md flex flex-col gap-xs text-xs text-text-2">
              <div className="flex items-center justify-between">
                <span className="font-medium text-text">Detected Environment</span>
                <StatusPill tone="neutral">
                  {isDesktop ? 'Desktop AppImage' : 'Browser / Container'}
                </StatusPill>
              </div>
              <p className="text-text-3">
                This guided wizard will configure your administrator credentials, local network
                access, and verify service readiness.
              </p>
            </div>

            <div className="flex justify-end mt-xs">
              <Button
                onClick={() => {
                  markStepComplete('welcome')
                  setStep('admin')
                }}
                className="w-full"
              >
                Begin Installation
              </Button>
            </div>
          </div>
        )}

        {/* ── STEP 2: ADMINISTRATOR ACCOUNT ─────────────────────────── */}
        {step === 'admin' && (
          <div className="flex flex-col gap-md" data-testid="step-admin">
            <div>
              <h2 className="text-xl font-serif font-bold text-text mb-4xs">
                Administrator Account
              </h2>
              <p className="text-xs text-text-3">
                Create the master account used to manage your library, collections, and settings.
              </p>
            </div>

            {backendStatus?.isSetup ? (
              <div className="rounded-lg border border-success/30 bg-success/10 p-md flex flex-col gap-sm">
                <div className="text-sm text-success font-medium">
                  Administrator account is already initialized on this server.
                </div>
                <Button
                  onClick={() => {
                    markStepComplete('admin')
                    setStep('network')
                  }}
                  className="w-full"
                >
                  Continue to Network Setup
                </Button>
              </div>
            ) : (
              <>
                {adminError && (
                  <div
                    id="setup-error"
                    className="rounded border border-error bg-surface p-sm text-sm text-error"
                    role="alert"
                  >
                    {adminError}
                  </div>
                )}

                <form onSubmit={handleAdminSubmit} className="flex flex-col gap-md">
                  <Input
                    label="Admin Username"
                    id="username"
                    type="text"
                    required
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder="e.g. librarian"
                  />

                  <Input
                    label="Email Address"
                    id="email"
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="admin@example.com"
                  />

                  <Input
                    ref={passwordRef}
                    label="Password"
                    id="password"
                    type="password"
                    required
                    minLength={8}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    aria-invalid={adminError?.includes('Password') ? true : undefined}
                    aria-describedby={adminError ? 'setup-error' : undefined}
                    placeholder="••••••••"
                  />

                  <Input
                    ref={confirmPasswordRef}
                    label="Confirm Password"
                    id="confirmPassword"
                    type="password"
                    required
                    minLength={8}
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    aria-invalid={adminError?.startsWith('Passwords do not match') ? true : undefined}
                    aria-describedby={
                      adminError?.startsWith('Passwords do not match') ? 'setup-error' : undefined
                    }
                    placeholder="••••••••"
                  />

                  <div className="flex items-center justify-between gap-sm mt-xs">
                    <Button
                      type="button"
                      variant="secondary"
                      onClick={() => setStep('welcome')}
                    >
                      Back
                    </Button>
                    <Button type="submit" disabled={adminLoading}>
                      {adminLoading ? 'Initializing…' : 'Initialize Alexandryn'}
                    </Button>
                  </div>
                </form>
              </>
            )}
          </div>
        )}

        {/* ── STEP 3: NETWORK SELECTION ─────────────────────────────── */}
        {step === 'network' && (
          <form
            onSubmit={handleNetworkSubmit}
            className="flex flex-col gap-md"
            data-testid="step-network"
          >
            <div>
              <h2 className="text-xl font-serif font-bold text-text mb-4xs">
                Network Configuration
              </h2>
              <p className="text-xs text-text-3">
                Select how readers and devices will connect to your library. Local Network (LAN) is
                recommended.
              </p>
            </div>

            <div className="flex flex-col gap-sm">
              {/* Option 1: LAN (Selected by default) */}
              <div
                className={cx(
                  'rounded-lg border p-md transition-colors flex flex-col gap-xs',
                  networkMode === 'lan'
                    ? 'border-accent bg-accent-soft/30'
                    : 'border-border bg-surface hover:bg-surface-2',
                )}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-xs">
                    <input
                      type="radio"
                      id="network-mode-lan"
                      name="networkMode"
                      value="lan"
                      checked={networkMode === 'lan'}
                      onChange={() => setNetworkMode('lan')}
                      className="accent-accent cursor-pointer"
                    />
                    <label
                      htmlFor="network-mode-lan"
                      className="font-serif font-semibold text-text text-sm cursor-pointer"
                    >
                      Local Network (LAN)
                    </label>
                  </div>
                  <StatusPill tone="success">Recommended</StatusPill>
                </div>
                <p className="text-xs text-text-2 pl-md">
                  Allows phones, tablets, e-readers, and computers on your Wi-Fi or Ethernet to
                  discover and access your library with zero manual configuration.
                </p>
                {networkMode === 'lan' && (
                  <div className="mt-xs pl-md flex flex-col gap-4xs">
                    <Input
                      label="Local Hostname (.local)"
                      id="hostName"
                      type="text"
                      required
                      value={hostName}
                      onChange={(e) => setHostName(e.target.value)}
                      placeholder="alexandryn.local"
                    />
                    <span className="text-xs text-text-3">
                      Devices on your network can connect directly to http://{hostName || 'alexandryn.local'}
                    </span>
                  </div>
                )}
              </div>

              {/* Option 2: Loopback Only */}
              <div
                className={cx(
                  'rounded-lg border p-md transition-colors flex flex-col gap-xs',
                  networkMode === 'loopback'
                    ? 'border-accent bg-accent-soft/30'
                    : 'border-border bg-surface hover:bg-surface-2',
                )}
              >
                <div className="flex items-center gap-xs">
                  <input
                    type="radio"
                    id="network-mode-loopback"
                    name="networkMode"
                    value="loopback"
                    checked={networkMode === 'loopback'}
                    onChange={() => setNetworkMode('loopback')}
                    className="accent-accent cursor-pointer"
                  />
                  <label
                    htmlFor="network-mode-loopback"
                    className="font-serif font-semibold text-text text-sm cursor-pointer"
                  >
                    This Device Only (Loopback)
                  </label>
                </div>
                <p className="text-xs text-text-2 pl-md">
                  Restricts access strictly to http://127.0.0.1:8080. Other devices on your local
                  network will not be able to connect.
                </p>
              </div>
            </div>

            <div className="flex items-center justify-between gap-sm mt-xs">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setStep('admin')}
              >
                Back
              </Button>
              <Button type="submit">Continue to Service Checks</Button>
            </div>
          </form>
        )}

        {/* ── STEP 4: SERVICE INITIALIZATION & PROBE ───────────────── */}
        {step === 'services' && (
          <div className="flex flex-col gap-md" data-testid="step-services">
            <div>
              <h2 className="text-xl font-serif font-bold text-text mb-4xs">
                Service Initialization
              </h2>
              <p className="text-xs text-text-3">
                Validating database connection, authentication state, and network discovery
                listeners.
              </p>
            </div>

            <div className="flex flex-col gap-xs border border-border rounded-lg bg-surface p-sm">
              <div className="flex items-center justify-between p-xs border-b border-border/50">
                <span className="text-xs font-medium text-text">Database & Core Server</span>
                <StatusPill tone="success">Ready ✓</StatusPill>
              </div>

              <div className="flex items-center justify-between p-xs border-b border-border/50">
                <span className="text-xs font-medium text-text">Administrator Credentials</span>
                <StatusPill tone="success">Configured ✓</StatusPill>
              </div>

              <div className="flex items-center justify-between p-xs">
                <div>
                  <div className="text-xs font-medium text-text">
                    Network Listener ({networkMode.toUpperCase()})
                  </div>
                  <div className="text-xs text-text-3">
                    {networkMode === 'lan'
                      ? `http://${hostName}${networkStatus?.address ? ` (${networkStatus.address})` : ''}`
                      : 'http://127.0.0.1:8080'}
                  </div>
                </div>
                <StatusPill tone={networkMode === 'lan' ? 'success' : 'neutral'}>
                  {networkMode === 'lan' ? 'Active' : 'Loopback'}
                </StatusPill>
              </div>
            </div>

            <div className="rounded border border-border bg-surface-2 p-sm text-xs text-text-2">
              All baseline services are responding. You can continue directly to the setup checklist
              or review terminal commands for host firewall verification.
            </div>

            <div className="flex items-center justify-between gap-sm mt-xs flex-wrap">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setStep('network')}
              >
                Back
              </Button>
              <div className="flex items-center gap-xs">
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setStep('commands')}
                  className="text-xs"
                >
                  View Network Commands
                </Button>
                <Button
                  type="button"
                  onClick={() => {
                    markStepComplete('services')
                    setStep('verification')
                  }}
                >
                  Proceed to Checklist
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* ── STEP 5: COMMAND STEPS ─────────────────────────────────── */}
        {step === 'commands' && (
          <div className="flex flex-col gap-md" data-testid="step-commands">
            <div>
              <h2 className="text-xl font-serif font-bold text-text mb-4xs">
                Manual Command Steps
              </h2>
              <p className="text-xs text-text-3">
                When running behind strict firewalls or managing containerized deployments, execute
                these commands in your terminal and verify.
              </p>
            </div>

            <div className="flex flex-col gap-md">
              <CommandStepCard
                title="Allow Local Network Discovery (mDNS)"
                rationale="Firewalls (such as UFW) may block multicast DNS on UDP port 5353, preventing other devices from resolving alexandryn.local."
                target={isDesktop ? 'Host Terminal' : 'Host / Server Terminal'}
                command="sudo ufw allow 5353/udp"
                status={commandStatus['mdns'] || 'pending'}
                onVerify={handleVerifyMdns}
                verifying={verifyingCommand === 'mdns'}
                fallbackText="If firewall rules cannot be changed, access Alexandryn directly via your local IP address."
                onFallback={() => setCommandStatus('mdns', 'verified')}
              />

              <CommandStepCard
                title="Verify Container Status"
                rationale="Confirms that the Alexandryn server and PostgreSQL database containers are running and healthy."
                target="Docker Terminal"
                command="docker compose ps"
                status={commandStatus['docker'] || 'pending'}
                onVerify={handleVerifyDocker}
                verifying={verifyingCommand === 'docker'}
                fallbackText="Run 'docker compose logs -f' to inspect container logs."
                onFallback={() => setCommandStatus('docker', 'verified')}
              />
            </div>

            <div className="flex items-center justify-between gap-sm mt-xs">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setStep('services')}
              >
                Back
              </Button>
              <Button
                type="button"
                onClick={() => {
                  markStepComplete('commands')
                  setStep('verification')
                }}
              >
                Continue to Checklist
              </Button>
            </div>
          </div>
        )}

        {/* ── STEP 6: REQUIREMENT VERIFICATION SUMMARY ──────────────── */}
        {step === 'verification' && (
          <div className="flex flex-col gap-md" data-testid="step-verification">
            <div>
              <h2 className="text-xl font-serif font-bold text-text mb-4xs">
                Setup Verification Checklist
              </h2>
              <p className="text-xs text-text-3">
                All prerequisites and configurations have been verified for your deployment.
              </p>
            </div>

            <div className="flex flex-col gap-xs rounded-lg border border-border bg-surface p-sm">
              <div className="flex items-center justify-between p-xs border-b border-border/50">
                <span className="text-xs text-text">1. Core Engine & Database</span>
                <StatusPill tone="success">Verified ✓</StatusPill>
              </div>
              <div className="flex items-center justify-between p-xs border-b border-border/50">
                <span className="text-xs text-text">2. Administrator Account</span>
                <StatusPill tone="success">Active ✓</StatusPill>
              </div>
              <div className="flex items-center justify-between p-xs border-b border-border/50">
                <span className="text-xs text-text">
                  3. Network Mode: {networkMode === 'lan' ? 'LAN (alexandryn.local)' : 'Loopback'}
                </span>
                <StatusPill tone="success">Configured ✓</StatusPill>
              </div>
              <div className="flex items-center justify-between p-xs">
                <span className="text-xs text-text">4. Access Listener</span>
                <StatusPill tone="success">Ready ✓</StatusPill>
              </div>
            </div>

            <div className="flex items-center justify-between gap-sm mt-xs">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setStep('services')}
              >
                Back
              </Button>
              <Button
                type="button"
                onClick={() => {
                  markStepComplete('verification')
                  setStep('complete')
                }}
              >
                Finish Setup
              </Button>
            </div>
          </div>
        )}

        {/* ── STEP 7: SETUP COMPLETION ──────────────────────────────── */}
        {step === 'complete' && (
          <div className="flex flex-col gap-md text-center" data-testid="step-complete">
            <AlexAvatar size="lg" className="mx-auto mb-xs" />
            <h1 className="text-2xl font-serif font-bold text-text">
              Alexandryn is Ready!
            </h1>
            <p className="text-xs text-text-3">
              Your self-hosted library is now fully configured and accessible on your network.
            </p>

            <div className="rounded-lg border border-border bg-surface-2 p-md flex flex-col gap-xs text-left">
              <div className="text-xs text-text-3 font-medium">Access Address:</div>
              <div className="flex items-center justify-between gap-xs bg-surface border border-border rounded p-xs">
                <code className="text-xs font-mono font-medium text-text select-all">
                  {primaryUrl}
                </code>
                <button
                  type="button"
                  onClick={handleCopyUrl}
                  className="inline-flex items-center rounded px-sm py-4xs text-xs font-ui font-medium border border-border bg-surface hover:bg-surface-3 text-text transition-colors"
                >
                  {copiedUrl ? 'Copied!' : 'Copy URL'}
                </button>
              </div>
              <p className="text-xs text-text-2 mt-xs leading-relaxed">
                Open this URL from any web browser on your network to sign in and begin organizing
                your collection.
              </p>
            </div>

            <div className="mt-xs">
              <Button onClick={handleFinish} className="w-full">
                Enter Library
              </Button>
            </div>
          </div>
        )}
      </div>
    </main>
  )
}
