import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Modal } from '../../components/Modal/Modal'
import { SourceFormDialog } from '../../components/SourceFormDialog/SourceFormDialog'
import { SourceStatusBadge } from '../../components/SourceStatusBadge/SourceStatusBadge'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  useDeleteSource,
  useHealthCheckSource,
  useSources,
  type Source,
} from '../../data/sources'
import { ApiError } from '../../data/http'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

/**
 * Sources index screen at /sources (frontend-source-management.md FR-1, FR-7).
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
      // Error handled via mutation status
    }
  }

  return (
    <div className="flex flex-col gap-xl p-3xl">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-md">
        <div>
          <h1 className="text-3xl font-medium tracking-1 text-text">Sources</h1>
          <p className="text-sm text-text-2 mt-4xs">
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
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-lg">
            {sources.map((src) => {
              const isLocal = src.kind === 'local-folder'
              const configDisplay = isLocal
                ? src.config.basePath
                : src.config.baseUrl

              return (
                <div
                  key={src.id}
                  className="flex flex-col justify-between rounded-lg border border-border bg-surface p-lg shadow-sm transition-all hover:border-text-3"
                >
                  <div className="flex flex-col gap-md">
                    {/* Header line: icon + label */}
                    <div className="flex items-start justify-between gap-sm">
                      <div className="flex items-center gap-xs">
                        <span className="text-xl" aria-hidden="true">
                          {isLocal ? '📁' : '🌐'}
                        </span>
                        <div>
                          <h2 className="text-base font-medium text-text">{src.label}</h2>
                          <span className="text-3xs font-ui uppercase tracking-wide text-text-3">
                            {isLocal ? 'Local folder' : 'OPDS catalog'}
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Config path / URL */}
                    <p
                      className="text-xs font-mono text-text-2 truncate bg-surface-2 p-2xs rounded"
                      title={configDisplay}
                    >
                      {configDisplay}
                    </p>

                    {/* Health status badge */}
                    <SourceStatusBadge
                      health={src.health}
                      onCheckAgain={() => healthCheckMutation.mutate(src.id)}
                      isChecking={
                        healthCheckMutation.isPending &&
                        healthCheckMutation.variables === src.id
                      }
                    />

                    {/* Capabilities badges */}
                    <div className="flex flex-wrap items-center gap-4xs">
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
                  <div className="mt-lg flex items-center justify-between border-t border-border/50 pt-sm">
                    <div className="flex items-center gap-2xs">
                      <Button
                        variant="ghost"
                        onClick={() => setEditingSource(src)}
                        className="text-xs px-2xs py-4xs h-auto"
                      >
                        Edit
                      </Button>
                      <Button
                        variant="ghost"
                        onClick={() => setSourceToDelete(src)}
                        className="text-xs px-2xs py-4xs h-auto text-error hover:bg-error/10"
                      >
                        Remove
                      </Button>
                    </div>

                    <Link
                      to={`/sources/${src.id}`}
                      className={cx(
                        'text-xs font-medium text-text-2 hover:text-text px-2xs py-4xs rounded',
                        FOCUS_RING,
                      )}
                    >
                      Browse →
                    </Link>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Add Source Dialog */}
      <SourceFormDialog
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
      />

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

      {/* Delete Confirmation Modal (FR-7) */}
      <Modal
        open={Boolean(sourceToDelete)}
        onOpenChange={(open) => {
          if (!open) setSourceToDelete(null)
        }}
        title="Remove source"
        description={`Are you sure you want to remove "${sourceToDelete?.label}"? This will not delete any files on your disk or books in your library.`}
      >
        <div className="mt-md flex items-center justify-end gap-sm">
          <Button
            variant="ghost"
            type="button"
            onClick={() => setSourceToDelete(null)}
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
            {deleteMutation.isPending ? 'Removing...' : 'Remove source'}
          </Button>
        </div>
      </Modal>
    </div>
  )
}
