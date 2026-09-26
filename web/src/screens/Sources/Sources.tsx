import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Modal } from '../../components/Modal/Modal'
import { SourceFormDialog } from '../../components/SourceFormDialog/SourceFormDialog'
import { SourceStatusBadge } from '../../components/SourceStatusBadge/SourceStatusBadge'
import { Spinner } from '../../components/Spinner/Spinner'
import { useDeleteSource, useHealthCheckSource, useSources, type Source } from '../../data/sources'
import { ApiError } from '../../data/http'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

/**
 * Sources index screen at /sources.
 * Lists configured sources with live health status, capabilities badges,
 * create/edit form modal, and confirmation-gated deletion.
 */
export function Sources() {
  const { data: sources = [], error, isPending, refetch } = useSources()
  const deleteMutation = useDeleteSource()
  const healthCheckMutation = useHealthCheckSource()

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingSource, setEditingSource] = useState<Source | null>(null)
  const [sourceToDelete, setSourceToDelete] = useState<Source | null>(null)

  const handleDeleteConfirm = async () => {
    if (!sourceToDelete) return
    try {
      await deleteMutation.mutateAsync(sourceToDelete.id)
      setSourceToDelete(null)
    } catch {
      // Keep the dialog open; the error is shown inline below (audit
      // 0016 #142 — the failure used to be swallowed entirely).
    }
  }

  const closeDeleteDialog = () => {
    setSourceToDelete(null)
    deleteMutation.reset()
  }

  return (
    <div className="flex flex-col gap-xl p-3xl content-container-medium">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-md mb-xs">
        <div>
          <h1 className="screen-title text-text">Sources</h1>
          <p className="text-2xl text-text-2 mt-3xs">
            Connect local book folders and OPDS catalogs to browse and import into your library.
          </p>
        </div>

        <Button variant="primary" onClick={() => setIsCreateOpen(true)}>
          + Add source
        </Button>
      </div>

      {/* Screen reader live region */}
      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {!isPending && `${sources.length} sources loaded`}
      </div>

      {/* Main content */}
      <div>
        {isPending ? (
          <Spinner label="Loading sources" className="m-3xl" />
        ) : error ? (
          <ErrorState
            title="Something went wrong loading sources"
            description={error instanceof ApiError ? error.message : undefined}
            code={error instanceof ApiError ? error.code : undefined}
            correlationId={error instanceof ApiError ? error.correlationId : undefined}
            onRetry={() => void refetch()}
          />
        ) : sources.length === 0 ? (
          <EmptyState
            title="No sources configured"
            description="Add a local folder or an OPDS catalog to browse and import books."
            action={{
              label: 'Add source',
              onClick: () => setIsCreateOpen(true),
            }}
          />
        ) : (
          <div className="source-card-grid">
            {sources.map((src) => {
              const isLocal = src.kind === 'local-folder'
              const configDisplay = isLocal ? src.config.basePath : src.config.baseUrl
              const abbr = isLocal ? 'DIR' : 'OPDS'

              return (
                <div
                  key={src.id}
                  className="flex flex-col justify-between rounded-md border border-border bg-surface p-lg shadow-xs transition-colors hover:border-text-3"
                >
                  <div className="flex flex-col gap-sm">
                    <div className="flex items-start justify-between gap-sm">
                      <div className="flex items-center gap-sm">
                        <div className="source-avatar-sm bg-surface-3 flex items-center justify-center font-mono text-3xs text-text-2 shrink-0">
                          {abbr}
                        </div>
                        <div className="min-w-0">
                          <h2 className="text-sm font-semibold tracking-tight text-text truncate">{src.label}</h2>
                          <span className="text-3xs font-mono uppercase tracking-wider text-text-3">
                            {isLocal ? 'LOCAL FOLDER' : 'OPDS CATALOG'}
                          </span>
                        </div>
                      </div>

                      {/* Health status badge */}
                      <SourceStatusBadge
                        health={src.health}
                        onCheckAgain={() => healthCheckMutation.mutate(src.id)}
                        isChecking={
                          healthCheckMutation.isPending && healthCheckMutation.variables === src.id
                        }
                      />
                    </div>

                    {/* Config path / URL */}
                    <p
                      className="text-xs font-mono text-text-3 truncate mt-xs"
                      title={configDisplay}
                    >
                      {configDisplay}
                    </p>

                    {/* Capabilities badges */}
                    <div className="flex flex-wrap items-center gap-4xs mt-xs">
                      {src.capabilities.canList && (
                        <span className="inline-flex items-center rounded-3xs bg-surface-3 px-2xs py-4xs text-3xs font-ui text-text-2">
                          Browse
                        </span>
                      )}
                      {src.capabilities.canSearch && (
                        <span className="inline-flex items-center rounded-3xs bg-surface-3 px-2xs py-4xs text-3xs font-ui text-text-2">
                          Search
                        </span>
                      )}
                      {src.capabilities.canDownload && (
                        <span className="inline-flex items-center rounded-3xs bg-surface-3 px-2xs py-4xs text-3xs font-ui text-text-2">
                          Download
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Actions footer */}
                  <div className="mt-md flex items-center justify-between border-t border-border pt-sm">
                    <div className="flex items-center gap-2xs">
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditingSource(src)}
                        className="px-2xs min-w-11"
                      >
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => setSourceToDelete(src)}
                        className="px-2xs min-w-11 text-error hover:bg-error/10"
                      >
                        Remove
                      </Button>
                    </div>

                    <Link
                      to={`/sources/${src.id}`}
                      className={cx(
                        'text-xs font-medium text-accent hover:underline px-2xs py-4xs rounded',
                        FOCUS_RING,
                      )}
                    >
                      Browse →
                    </Link>
                  </div>
                </div>
              )
            })}

            {/* Prototype Add a Source Dashed Card */}
            <button
              type="button"
              onClick={() => setIsCreateOpen(true)}
              className={cx(
                'flex flex-col justify-center items-start gap-xs p-lg rounded-md border border-dashed border-border hover:border-text-2 bg-transparent text-left cursor-pointer transition-colors source-dashed-card',
                FOCUS_RING,
              )}
            >
              <div className="source-avatar-sm rounded-2xs border border-border flex items-center justify-center text-text-3 text-base">
                +
              </div>
              <div className="text-sm font-semibold text-text mt-2xs">Add a source</div>
              <div className="text-xs text-text-3 max-w-[26ch] leading-relaxed">
                OPDS, a folder on this machine, a remote library or cloud storage.
              </div>
            </button>
          </div>
        )}
      </div>

      {/* Add Source Dialog */}
      <SourceFormDialog open={isCreateOpen} onOpenChange={setIsCreateOpen} />

      {/* Edit Source Dialog */}
      {editingSource && (
        <SourceFormDialog
          open={Boolean(editingSource)}
          onOpenChange={(open) => {
            if (!open) setEditingSource(null)
          }}
          source={editingSource}
        />
      )}

      {/* Delete Confirmation Modal */}
      <Modal
        open={Boolean(sourceToDelete)}
        onOpenChange={(open) => {
          if (!open) closeDeleteDialog()
        }}
        title="Remove source"
        description={`Are you sure you want to remove "${sourceToDelete?.label}"? This will not delete any files on your disk or books in your library.`}
      >
        {deleteMutation.isError && (
          <div
            role="alert"
            className="mt-md rounded-md border border-error/20 bg-error/10 p-md text-sm text-error"
          >
            {deleteMutation.error instanceof ApiError
              ? deleteMutation.error.message
              : 'Could not remove the source. Check your connection and try again.'}
          </div>
        )}
        <div className="mt-md flex items-center justify-end gap-sm">
          <Button
            variant="ghost"
            type="button"
            onClick={closeDeleteDialog}
            disabled={deleteMutation.isPending}
          >
            Cancel
          </Button>
          <Button
            variant="primary"
            type="button"
            onClick={handleDeleteConfirm}
            disabled={deleteMutation.isPending}
            className="bg-error hover:bg-error/90 text-white"
          >
            {deleteMutation.isPending
              ? 'Removing...'
              : deleteMutation.isError
                ? 'Try again'
                : 'Remove source'}
          </Button>
        </div>
      </Modal>
    </div>
  )
}
