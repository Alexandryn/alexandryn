import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Button } from '../../components/Button'
import { getCurrentUser } from '../../data/auth'
import { MfaSetupModal } from '../Auth/MfaSetupModal'

export function AccountSettings() {
  const user = getCurrentUser()
  const [isMfaModalOpen, setIsMfaModalOpen] = useState(false)
  const [mfaSuccessNotice, setMfaSuccessNotice] = useState(false)

  const roleLabel = user?.role === 'admin' ? 'Administrator' : 'Reader'

  return (
    <div className="p-xl max-w-[48rem] mx-auto flex flex-col gap-lg">
      <div>
        <h1 className="text-2xl font-ui font-medium text-text">Account Settings</h1>
        <p className="text-sm text-text-2 mt-4xs max-w-[42rem]">
          Manage your personal Alexandryn account profile and security.
        </p>
      </div>

      {mfaSuccessNotice && (
        <div
          role="status"
          className="rounded-md bg-success/10 border border-success/20 p-md text-sm text-success flex items-center justify-between"
        >
          <span>Two-factor authentication has been successfully configured.</span>
          <button
            type="button"
            onClick={() => setMfaSuccessNotice(false)}
            className="text-success font-bold text-xs hover:underline cursor-pointer"
            aria-label="Dismiss message"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* User Identity Card */}
      <section
        aria-labelledby="profile-heading"
        className="rounded-md border border-border bg-surface p-lg flex flex-col gap-md shadow-sm"
      >
        <h2 id="profile-heading" className="text-lg font-medium text-text">
          Profile
        </h2>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-md">
          <div className="flex flex-col gap-4xs">
            <span className="text-xs uppercase font-medium tracking-wide text-text-3">
              Username
            </span>
            <span className="font-mono text-sm text-text font-medium">
              {user?.username || '—'}
            </span>
          </div>

          <div className="flex flex-col gap-4xs">
            <span className="text-xs uppercase font-medium tracking-wide text-text-3">
              Email Address
            </span>
            <span className="text-sm text-text">
              {user?.email || '—'}
            </span>
          </div>

          <div className="flex flex-col gap-4xs">
            <span className="text-xs uppercase font-medium tracking-wide text-text-3">
              Account Role
            </span>
            <div>
              <span className="inline-flex items-center px-sm py-4xs rounded-full text-xs font-medium border border-border bg-surface-2 text-text-2">
                {roleLabel}
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* Security & MFA Card */}
      <section
        aria-labelledby="security-heading"
        className="rounded-md border border-border bg-surface p-lg flex flex-col gap-md shadow-sm"
      >
        <div className="flex flex-col gap-4xs">
          <h2 id="security-heading" className="text-lg font-medium text-text">
            Two-Factor Authentication
          </h2>
          <p className="text-sm text-text-2">
            Add an extra layer of security to your account using an authenticator application (such as 1Password, Google Authenticator, or Bitwarden).
          </p>
        </div>

        <div>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setIsMfaModalOpen(true)}
          >
            Configure Authenticator App
          </Button>
        </div>
      </section>

      {/* Connected Devices Shortcut */}
      <section
        aria-labelledby="devices-heading"
        className="rounded-md border border-border bg-surface p-lg flex flex-col gap-md shadow-sm"
      >
        <div className="flex flex-col gap-4xs">
          <h2 id="devices-heading" className="text-lg font-medium text-text">
            Connected Devices
          </h2>
          <p className="text-sm text-text-2">
            View and manage tablets, phones, and computers connected to your account for reading progress sync.
          </p>
        </div>

        <div>
          <Link
            to="/settings/devices"
            className="inline-flex items-center text-sm font-medium text-accent hover:underline cursor-pointer"
          >
            View and manage connected devices →
          </Link>
        </div>
      </section>

      <MfaSetupModal
        isOpen={isMfaModalOpen}
        onClose={() => setIsMfaModalOpen(false)}
        onSuccess={() => setMfaSuccessNotice(true)}
      />
    </div>
  )
}
