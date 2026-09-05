import { useState, useId } from 'react'
import { Button } from '../../components/Button'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  isAdminNetworkStatus,
  useNetworkStatus,
  useUpdateNetworkSettings,
} from '../../data/network'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { DevicePairingModal } from '../Network/DevicePairingModal'

export function getReachabilityDescription(reachability: string): string {
  switch (reachability) {
    case 'loopback':
    case 'local_only':
      return 'Alexandryn is only accessible from this computer.'
    case 'private':
    case 'lan':
    case 'local_network':
      return 'Accessible from devices on your local network.'
    case 'public':
    case 'internet':
      return 'Accessible from the internet.'
    default:
      return reachability
  }
}

export function getTLSDescription(tlsMode: string): string {
  switch (tlsMode) {
    case 'none':
      return 'Traffic on your local network is unencrypted. Anyone with access to your Wi-Fi or router can see the books you read and the pages you view, unless you are using a reverse proxy that provides TLS.'
    case 'static':
      return 'Encrypted with a custom TLS certificate.'
    case 'acme':
      return "Encrypted with an automatic Let's Encrypt TLS certificate."
    default:
      return tlsMode
  }
}

export function NetworkSettings() {
  const { data: status, isLoading, error } = useNetworkStatus()
  const updateSettingsMutation = useUpdateNetworkSettings()

  const [isAdvancedOpen, setIsAdvancedOpen] = useState(false)
  const [isPairingModalOpen, setIsPairingModalOpen] = useState(false)

  // Editable settings fields
  const [hostNameInput, setHostNameInput] = useState<string | null>(null)
  const [rememberDaysInput, setRememberDaysInput] = useState<number | null>(null)
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [toastError, setToastError] = useState<string | null>(null)

  const hostNameId = useId()
  const rememberDaysId = useId()

  const isAdmin = status ? isAdminNetworkStatus(status) : false
  const currentHostName =
    hostNameInput !== null
      ? hostNameInput
      : isAdmin && status && 'hostName' in status
        ? status.hostName
        : 'alexandryn.local'

  const currentRememberDays = rememberDaysInput !== null ? rememberDaysInput : 30

  const handleSaveSettings = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaveSuccess(false)
    setToastError(null)

    // Optimistic values to rollback if needed
    const previousHostName = hostNameInput
    const previousRememberDays = rememberDaysInput

    try {
      await updateSettingsMutation.mutateAsync({
        hostName: currentHostName,
        rememberDeviceDays: currentRememberDays,
      })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 4000)
    } catch (err: unknown) {
      // Rollback
      setHostNameInput(previousHostName)
      setRememberDaysInput(previousRememberDays)
      const msg = err instanceof Error ? err.message : 'Failed to save network settings'
      setToastError(msg)
    }
  }

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-3xl gap-md">
        <Spinner label="Loading network status" />
        <p className="text-xs text-text-2">Fetching network configuration…</p>
      </div>
    )
  }

  if (error || !status) {
    return (
      <div className="p-xl max-w-3xl mx-auto">
        <div className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error">
          Failed to load network status. The server may be unreachable.
        </div>
      </div>
    )
  }

  const certStatus =
    status.tlsMode === 'static' || status.tlsMode === 'acme' ? 'configured' : 'not configured'

  return (
    <div className="p-xl max-w-3xl mx-auto flex flex-col gap-2xl">
      {/* Toast feedback */}
      {toastError && (
        <div
          role="alert"
          className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error flex items-center justify-between"
        >
          <span>{toastError}</span>
          <button
            type="button"
            onClick={() => setToastError(null)}
            className="text-error font-bold text-xs"
            aria-label="Dismiss error"
          >
            Dismiss
          </button>
        </div>
      )}

      {saveSuccess && (
        <div
          role="status"
          className="rounded-md bg-success/10 border border-success/20 p-md text-sm text-success"
        >
          Network settings saved successfully.
        </div>
      )}

      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-ui font-medium text-text">Network Access</h1>
          <p className="text-sm text-text-2 mt-4xs">
            Manage local network reachability and connected devices.
          </p>
        </div>
        <Button
          variant="primary"
          onClick={() => setIsPairingModalOpen(true)}
          className="shrink-0"
        >
          Pair a new device
        </Button>
      </div>

      {/* Network Status Card */}
      <section aria-labelledby="status-heading" className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-lg shadow-sm">
        <h2 id="status-heading" className="text-lg font-ui font-medium text-text">
          Server Status
        </h2>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-lg text-sm">
          <div className="flex flex-col gap-4xs">
            <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Reachability</span>
            <span data-testid="reachability-val" className="font-medium text-text">
              {getReachabilityDescription(status.reachability)}
            </span>
          </div>

          <div className="flex flex-col gap-4xs">
            <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Server Address</span>
            <span data-testid="server-address-val" className="font-mono text-text">
              {status.address}
            </span>
          </div>

          <div className="flex flex-col gap-4xs md:col-span-2">
            <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Encryption & Security</span>
            <span data-testid="tls-mode-desc" className="text-text-2 leading-relaxed">
              {getTLSDescription(status.tlsMode)}
            </span>
          </div>

          {/* Static "Authentication is always on" row — NO toggle role */}
          <div className="flex flex-col gap-4xs md:col-span-2 pt-sm border-t border-border">
            <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Access Protection</span>
            <div className="flex items-center gap-xs">
              <span className="inline-block w-2xs h-2xs rounded-full bg-success" aria-hidden="true" />
              <span data-testid="auth-always-on" className="font-medium text-text">
                Authentication is always on
              </span>
            </div>
            <p className="text-xs text-text-2">
              Every request from other devices must be authenticated with valid user credentials.
            </p>
          </div>
        </div>
      </section>

      {/* Settings Form */}
      <section aria-labelledby="settings-heading" className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-lg shadow-sm">
        <h2 id="settings-heading" className="text-lg font-ui font-medium text-text">
          Network Settings
        </h2>

        <form onSubmit={handleSaveSettings} className="flex flex-col gap-lg">
          <div className="flex flex-col gap-xs">
            <label htmlFor={hostNameId} className="text-sm font-medium text-text">
              Local Network Name (mDNS)
            </label>
            <p className="text-xs text-text-2">
              The .local hostname used by devices to discover this library on your home network.
            </p>
            <input
              id={hostNameId}
              type="text"
              value={currentHostName}
              onChange={(e) => setHostNameInput(e.target.value)}
              placeholder="alexandryn.local"
              className={cx(
                'rounded-md border border-border bg-surface px-md py-xs text-sm text-text font-mono max-w-md',
                FOCUS_RING,
              )}
            />
          </div>

          <div className="flex flex-col gap-xs">
            <label htmlFor={rememberDaysId} className="text-sm font-medium text-text">
              Remember Devices (Days)
            </label>
            <p className="text-xs text-text-2">
              Paired devices stay logged in without re-authenticating for this number of days (1–90).
            </p>
            <input
              id={rememberDaysId}
              type="number"
              min={1}
              max={90}
              value={currentRememberDays}
              onChange={(e) => setRememberDaysInput(Number(e.target.value))}
              className={cx(
                'rounded-md border border-border bg-surface px-md py-xs text-sm text-text max-w-xs',
                FOCUS_RING,
              )}
            />
          </div>

          <div>
            <Button
              type="submit"
              variant="primary"
              size="sm"
              disabled={updateSettingsMutation.isPending}
            >
              {updateSettingsMutation.isPending ? 'Saving…' : 'Save Settings'}
            </Button>
          </div>
        </form>
      </section>

      {/* Advanced Disclosure */}
      <section aria-labelledby="advanced-heading" className="rounded-lg bg-surface border border-border p-xl flex flex-col gap-md shadow-sm">
        <div className="flex items-center justify-between">
          <h2 id="advanced-heading" className="text-lg font-ui font-medium text-text">
            Advanced Configuration
          </h2>
          <Button
            variant="ghost"
            size="sm"
            aria-expanded={isAdvancedOpen}
            aria-controls="advanced-disclosure-panel"
            onClick={() => setIsAdvancedOpen((prev) => !prev)}
          >
            {isAdvancedOpen ? 'Hide Advanced' : 'Show Advanced'}
          </Button>
        </div>

        {isAdvancedOpen && (
          <div
            id="advanced-disclosure-panel"
            className="pt-md border-t border-border flex flex-col gap-md text-sm"
          >
            <div className="grid grid-cols-1 md:grid-cols-2 gap-md">
              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase tracking-7 text-text-3 font-medium">Bind Address</span>
                <span data-testid="advanced-bind-address" className="font-mono text-text">
                  {status.address}
                </span>
              </div>

              <div className="flex flex-col gap-4xs">
                <span className="text-xs uppercase tracking-7 text-text-3 font-medium">TLS Certificate</span>
                <span data-testid="advanced-tls-cert" className="font-medium text-text">
                  {certStatus}
                </span>
              </div>

              {status.tlsMode === 'acme' && isAdmin && 'acmeDomain' in status && status.acmeDomain && (
                <div className="flex flex-col gap-4xs">
                  <span className="text-xs uppercase tracking-7 text-text-3 font-medium">ACME Domain</span>
                  <span data-testid="advanced-acme-domain" className="font-mono text-text">
                    {status.acmeDomain}
                  </span>
                </div>
              )}
            </div>

            {status.tlsMode === 'acme' && (
              <p className="text-xs text-text-2 italic">
                Subject to Let's Encrypt Terms of Service.
              </p>
            )}

            <p className="text-xs text-text-2 bg-surface-2 p-sm rounded-md border border-border">
              Changes to bind address, TLS certificates, and domain require editing the config file and restarting the server.
            </p>
          </div>
        )}
      </section>

      {/* Device Pairing Modal */}
      <DevicePairingModal
        open={isPairingModalOpen}
        onOpenChange={setIsPairingModalOpen}
      />
    </div>
  )
}
