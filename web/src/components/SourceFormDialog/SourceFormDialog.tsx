import { useState, type FormEvent } from 'react'
import { Button } from '../Button/Button'
import { Input } from '../Input/Input'
import { Modal } from '../Modal/Modal'
import { SegmentedControl } from '../SegmentedControl/SegmentedControl'
import { useCreateSource, useUpdateSource, type Source, type SourceKind } from '../../data/sources'
import { ApiError } from '../../data/http'

interface AlexandrynWindow {
  alexandryn?: {
    source?: {
      pickLocalFolder?: () => Promise<{ path: string } | null>
    }
  }
}

export interface SourceFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  source?: Source
  onSuccess?: () => void
}

interface SourceFormContentProps {
  source?: Source
  onClose: () => void
  onSuccess?: () => void
}

function SourceFormContent({ source, onClose, onSuccess }: SourceFormContentProps) {
  const isEditing = Boolean(source)

  const [label, setLabel] = useState(source?.label ?? '')
  const [kind, setKind] = useState<SourceKind>(source?.kind ?? 'local-folder')
  const [basePath, setBasePath] = useState(source?.config.basePath ?? '')
  const [baseUrl, setBaseUrl] = useState(source?.config.baseUrl ?? '')
  const [requiresAuth, setRequiresAuth] = useState(Boolean(source?.hasCredential))
  const [isReplacingCred, setIsReplacingCred] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [validationError, setValidationError] = useState<string | null>(null)

  const createMutation = useCreateSource()
  const updateMutation = useUpdateSource()

  const isPending = createMutation.isPending || updateMutation.isPending

  const isNativePickerAvailable =
    typeof window !== 'undefined' &&
    'alexandryn' in window &&
    Boolean((window as unknown as AlexandrynWindow).alexandryn?.source?.pickLocalFolder)

  const handleNativeBrowse = async () => {
    try {
      const result = await (
        window as unknown as AlexandrynWindow
      ).alexandryn?.source?.pickLocalFolder?.()
      if (result && typeof result.path === 'string') {
        setBasePath(result.path)
        if (validationError) setValidationError(null)
      }
    } catch {
      // Ignore user cancellation or IPC errors
    }
  }

  const isCredentialSubformVisible =
    kind === 'opds' &&
    (!isEditing ? requiresAuth : !source?.hasCredential ? requiresAuth : isReplacingCred)

  const isHttpWarningVisible =
    isCredentialSubformVisible &&
    baseUrl.trim() !== '' &&
    !baseUrl.trim().toLowerCase().startsWith('https://')

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()

    const trimmedLabel = label.trim()
    if (!trimmedLabel) {
      setValidationError('Source label is required.')
      return
    }
    if (trimmedLabel.length > 100) {
      setValidationError('Source label cannot exceed 100 characters.')
      return
    }

    if (kind === 'local-folder') {
      const trimmedPath = basePath.trim()
      if (!trimmedPath) {
        setValidationError('Folder path is required.')
        return
      }
    } else if (kind === 'opds') {
      const trimmedUrl = baseUrl.trim()
      if (!trimmedUrl) {
        setValidationError('Catalog URL is required.')
        return
      }
      try {
        const parsed = new URL(trimmedUrl)
        if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
          setValidationError('Catalog URL must start with http:// or https://')
          return
        }
      } catch {
        setValidationError('Catalog URL is not a valid URL.')
        return
      }

      if (isCredentialSubformVisible) {
        if (!username.trim()) {
          setValidationError('Username is required for authentication.')
          return
        }
        if (!password.trim()) {
          setValidationError('Password is required for authentication.')
          return
        }
      }
    }

    setValidationError(null)

    try {
      if (isEditing && source) {
        const updatePayload: {
          label: string
          config: { basePath?: string; baseUrl?: string }
          credential?: { username: string; password: string }
        } = {
          label: trimmedLabel,
          config:
            kind === 'local-folder' ? { basePath: basePath.trim() } : { baseUrl: baseUrl.trim() },
        }

        if (isCredentialSubformVisible && username.trim() && password.trim()) {
          updatePayload.credential = {
            username: username.trim(),
            password: password.trim(),
          }
        }

        await updateMutation.mutateAsync({
          id: source.id,
          input: updatePayload,
        })
      } else {
        await createMutation.mutateAsync({
          label: trimmedLabel,
          kind,
          config:
            kind === 'local-folder' ? { basePath: basePath.trim() } : { baseUrl: baseUrl.trim() },
          credential:
            isCredentialSubformVisible && username.trim() && password.trim()
              ? { username: username.trim(), password: password.trim() }
              : undefined,
        })
      }

      onClose()
      onSuccess?.()
    } catch {
      // Error surfaced through mutation status
    }
  }

  const activeError = createMutation.error || updateMutation.error
  const errorMessage =
    activeError instanceof ApiError
      ? activeError.message
      : activeError
        ? 'Failed to save source.'
        : null

  return (
    <form onSubmit={handleSubmit} className="mt-md flex flex-col gap-md">
      <Input
        label="Source label"
        value={label}
        onChange={(e) => {
          setLabel(e.target.value)
          if (validationError) setValidationError(null)
        }}
        placeholder="e.g. Personal Library, Standard Ebooks"
        maxLength={100}
        required
      />

      {!isEditing && (
        <div className="flex flex-col gap-2xs">
          <span className="text-xs font-medium text-text-2">Source type</span>
          <SegmentedControl
            aria-label="Source type"
            value={kind}
            onValueChange={(val) => {
              setKind(val as SourceKind)
              if (validationError) setValidationError(null)
            }}
            options={[
              { value: 'local-folder', label: 'Local folder' },
              { value: 'opds', label: 'OPDS catalog' },
            ]}
          />
        </div>
      )}

      {kind === 'local-folder' ? (
        <div className="flex flex-col gap-2xs">
          <div className="flex items-end gap-xs">
            <div className="flex-1">
              <Input
                label="Folder path"
                value={basePath}
                onChange={(e) => {
                  setBasePath(e.target.value)
                  if (validationError) setValidationError(null)
                }}
                placeholder="/path/to/books or C:\Books"
                required
              />
            </div>
            {isNativePickerAvailable && (
              <Button
                type="button"
                variant="secondary"
                onClick={handleNativeBrowse}
                className="mb-4xs"
              >
                Browse...
              </Button>
            )}
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-md">
          <Input
            label="Catalog base URL"
            value={baseUrl}
            onChange={(e) => {
              setBaseUrl(e.target.value)
              if (validationError) setValidationError(null)
            }}
            placeholder="https://opds.example.org/catalog"
            required
          />

          {/* Credential form logic */}
          {isEditing && source?.hasCredential && !isReplacingCred ? (
            <div className="flex items-center justify-between rounded-md border border-border bg-surface-2 p-sm">
              <span className="text-xs text-text-2 font-ui">Password set</span>
              <Button
                type="button"
                variant="ghost"
                onClick={() => setIsReplacingCred(true)}
                className="text-xs px-2xs py-4xs h-auto"
              >
                Replace
              </Button>
            </div>
          ) : (
            <div className="flex flex-col gap-sm">
              {!isEditing && (
                <label className="flex items-center gap-xs text-xs font-ui text-text cursor-pointer">
                  <input
                    type="checkbox"
                    checked={requiresAuth}
                    onChange={(e) => setRequiresAuth(e.target.checked)}
                    className="rounded border-border"
                  />
                  Requires a username and password
                </label>
              )}

              {isEditing && !source?.hasCredential && !requiresAuth && (
                <Button
                  type="button"
                  variant="ghost"
                  onClick={() => setRequiresAuth(true)}
                  className="text-xs self-start"
                >
                  + Add username and password
                </Button>
              )}

              {isCredentialSubformVisible && (
                <div className="flex flex-col gap-sm rounded-md border border-border bg-surface-2 p-md">
                  {isHttpWarningVisible && (
                    <p className="text-xs text-warm font-ui" role="alert">
                      This source doesn't use HTTPS. Your password will be sent unencrypted.
                    </p>
                  )}

                  <Input
                    label="Username"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    autoComplete="username"
                    required
                  />

                  <Input
                    label="Password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="new-password"
                    required
                  />

                  {isEditing && isReplacingCred && (
                    <Button
                      type="button"
                      variant="ghost"
                      onClick={() => {
                        setIsReplacingCred(false)
                        setUsername('')
                        setPassword('')
                      }}
                      className="text-xs self-start"
                    >
                      Cancel replace
                    </Button>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {validationError && (
        <p className="text-xs text-error font-ui" role="alert">
          {validationError}
        </p>
      )}

      {errorMessage && (
        <p className="text-xs text-error font-ui" role="alert">
          {errorMessage}
        </p>
      )}

      <div className="mt-sm flex items-center justify-end gap-sm">
        <Button variant="ghost" type="button" onClick={onClose} disabled={isPending}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={isPending}>
          {isPending
            ? isEditing
              ? 'Saving...'
              : 'Adding...'
            : isEditing
              ? 'Save changes'
              : 'Add source'}
        </Button>
      </div>
    </form>
  )
}

/**
 * SourceFormDialog (frontend-source-management.md FR-2, FR-4, FR-5):
 * Unified create/edit dialog with platform-aware local folder picker,
 * HTTPS plain-text basic auth warning, and write-only credential replacement.
 */
export function SourceFormDialog({ open, onOpenChange, source, onSuccess }: SourceFormDialogProps) {
  const isEditing = Boolean(source)

  return (
    <Modal
      open={open}
      onOpenChange={onOpenChange}
      title={isEditing ? 'Edit source' : 'Add source'}
      description={
        isEditing
          ? 'Update the connection settings for this book source.'
          : 'Configure a new local folder or OPDS catalog source.'
      }
    >
      {open && (
        <SourceFormContent
          source={source}
          onClose={() => onOpenChange(false)}
          onSuccess={onSuccess}
        />
      )}
    </Modal>
  )
}
