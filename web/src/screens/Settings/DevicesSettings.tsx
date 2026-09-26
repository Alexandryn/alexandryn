import { useState } from 'react'
import * as RadixDialog from '@radix-ui/react-dialog'
import { Button } from '../../components/Button'
import { Spinner } from '../../components/Spinner/Spinner'
import { useDevices, useRevokeDevice, type DeviceItem } from '../../data/devices'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { useAnnouncedText } from '../../lib/useAnnouncedText'
import { buildKindLine, formatRelativeTime, isActiveDot } from './deviceUtils'

export function DevicesSettings() {
  const { data, isLoading, error, refetch } = useDevices()
  const revokeMutation = useRevokeDevice()

  const [targetDevice, setTargetDevice] = useState<DeviceItem | null>(null)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const liveRegionRef = useAnnouncedText<HTMLDivElement>(announcement)

  // Filter out revoked devices
  const activeDevices = (data?.devices ?? []).filter((d) => !d.revokedAt)

  const handleConfirmRevoke = async () => {
    if (!targetDevice) return
    const deviceToRevoke = targetDevice
    setTargetDevice(null)
    setErrorMessage(null)

    try {
      await revokeMutation.mutateAsync(deviceToRevoke.id)
      setAnnouncement(`Revoked ${deviceToRevoke.label}`)
    } catch (err: unknown) {
      const msg =
        err instanceof Error
          ? err.message
          : `Failed to revoke ${deviceToRevoke.label}`
      setErrorMessage(msg)
      setAnnouncement(`Failed to revoke ${deviceToRevoke.label}`)
      // Re-fetch on 404/409 or network error
      void refetch()
    }
  }

  return (
    // max-w-[48rem] not max-w-3xl: --spacing-3xl collides with Tailwind's max-w-3xl key
    <div className="p-xl max-w-[48rem] mx-auto flex flex-col gap-lg">
      {/* Polite live region for screen-reader announcements */}
      <div
        role="status"
        aria-live="polite"
        className="sr-only"
        ref={liveRegionRef}
      />

      <div>
        <h1 className="text-2xl font-ui font-medium text-text">Devices</h1>
        <p className="text-sm text-text-2 mt-4xs max-w-[42rem]"> {/* --spacing-2xl collision */}
          Devices paired with this library. Revoking a device prevents it from syncing reading progress and annotations.
        </p>
      </div>

      {/* Inline error message if revocation failed */}
      {errorMessage && (
        <div
          role="alert"
          className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error flex items-center justify-between"
        >
          <span>{errorMessage}</span>
          <button
            type="button"
            onClick={() => setErrorMessage(null)}
            className="text-error font-bold text-xs hover:underline"
            aria-label="Dismiss error"
          >
            Dismiss
          </button>
        </div>
      )}

      {isLoading && (
        <div className="flex flex-col items-center justify-center p-3xl gap-md">
          <Spinner label="Loading devices" />
          <p className="text-xs text-text-2">Fetching connected devices…</p>
        </div>
      )}

      {error && (
        <div className="rounded-md bg-error/10 border border-error/20 p-md text-sm text-error flex items-center justify-between">
          <span>Failed to load devices list.</span>
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      )}

      {/* Zero rows treated as error/loading indicator, not empty state */}
      {!isLoading && !error && activeDevices.length === 0 && (
        <div
          role="alert"
          className="rounded-md bg-surface-2 border border-border p-md text-sm text-text-2 flex items-center justify-between"
        >
          <span>No connected devices reported by server.</span>
          <Button variant="secondary" size="sm" onClick={() => void refetch()}>
            Retry
          </Button>
        </div>
      )}

      {!isLoading && !error && activeDevices.length > 0 && (
        <div className="border border-border rounded-xl bg-surface overflow-hidden divide-y divide-border">
          {activeDevices.map((device) => {
            const active = isActiveDot(device.lastSeenAt)
            return (
              <div
                key={device.id}
                data-testid={`device-row-${device.id}`}
                className="flex items-center gap-md px-lg py-md hover:bg-surface-2/50 transition-colors"
              >
                {/* Status Dot */}
                <div
                  data-testid="device-status-dot"
                  className={cx(
                    'w-2 h-2 rounded-full flex-none',
                    active ? 'bg-success' : 'bg-text-3',
                  )}
                  title={active ? 'Active' : 'Idle'}
                  aria-hidden="true"
                />

                {/* Device Label and Kind */}
                <div className="w-48 flex-none">
                  <div className="text-sm font-medium text-text truncate">
                    {device.label}
                  </div>
                  <div className="text-xs text-text-3">
                    {buildKindLine(device.deviceClass, device.enrolledVia)}
                  </div>
                </div>

                {/* Last Seen and Last Synced */}
                <div className="flex-1 flex flex-col sm:flex-row sm:items-center gap-xs sm:gap-lg text-xs">
                  <span className="text-text-2">
                    {formatRelativeTime(device.lastSeenAt)}
                  </span>
                  <span className="text-text-3">
                    {formatRelativeTime(device.lastSyncedAt, undefined, 'Synced')}
                  </span>
                </div>

                {/* Revoke Action */}
                <div className="flex-none">
                  <button
                    type="button"
                    onClick={() => setTargetDevice(device)}
                    className={cx(
                      'text-xs text-error hover:underline cursor-pointer px-xs py-2xs rounded',
                      FOCUS_RING,
                    )}
                    aria-label={`Revoke ${device.label}`}
                  >
                    Revoke
                  </button>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Confirmation Dialog */}
      <RadixDialog.Root
        open={targetDevice !== null}
        onOpenChange={(open) => {
          if (!open) {
            setTargetDevice(null)
          }
        }}
      >
        <RadixDialog.Portal>
          <RadixDialog.Overlay className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 animate-fade-in" />
          <RadixDialog.Content
            className={cx(
              'fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-50',
              // max-w-[28rem] not max-w-md: --spacing-md collides with Tailwind's max-w-md key
              'w-full max-w-[28rem] bg-surface border border-border rounded-xl p-xl shadow-xl flex flex-col gap-md',
              FOCUS_RING,
            )}
          >
            <RadixDialog.Title className="text-md font-semibold text-text">
              Revoke {targetDevice?.label}?
            </RadixDialog.Title>
            <RadixDialog.Description className="text-xs text-text-2 leading-relaxed">
              Are you sure you want to revoke {targetDevice?.label}? Revoking this device prevents it from syncing reading progress and annotations.
            </RadixDialog.Description>
            <div className="flex items-center justify-end gap-sm mt-md pt-sm border-t border-border">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setTargetDevice(null)}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                className="bg-error hover:bg-error/90 text-white"
                size="sm"
                onClick={() => void handleConfirmRevoke()}
              >
                Revoke
              </Button>
            </div>
          </RadixDialog.Content>
        </RadixDialog.Portal>
      </RadixDialog.Root>
    </div>
  )
}
