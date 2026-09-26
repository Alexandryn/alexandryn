import { useRef, useState, type FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Input } from '../../components/Input/Input'
import { Modal } from '../../components/Modal/Modal'
import { Spinner } from '../../components/Spinner/Spinner'
import { useCollections, useCreateCollection, type CollectionSummary } from '../../data/collections'
import { ApiError } from '../../data/http'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { FolderIcon, ChevronRightIcon } from '../../components/Icon'

/**
 * Collections index screen at /collections.
 * Displays all collections as tiles with book counts and provides a modal to create new ones.
 */
export function Collections() {
  const { data: collections = [], error, isPending, refetch } = useCollections()
  const createMutation = useCreateCollection()

  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [newCollectionName, setNewCollectionName] = useState('')
  const [validationError, setValidationError] = useState<string | null>(null)
  const nameInputRef = useRef<HTMLInputElement>(null)

  const handleOpenCreateModal = () => {
    setNewCollectionName('')
    setValidationError(null)
    createMutation.reset()
    setIsCreateOpen(true)
  }

  const handleCreateSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const trimmed = newCollectionName.trim()

    if (!trimmed) {
      setValidationError('Collection name is required.')
      nameInputRef.current?.focus()
      return
    }
    if (trimmed.length > 100) {
      setValidationError('Collection name cannot exceed 100 characters.')
      nameInputRef.current?.focus()
      return
    }

    setValidationError(null)
    try {
      await createMutation.mutateAsync(trimmed)
      setIsCreateOpen(false)
      setNewCollectionName('')
    } catch {
      // Error is handled via createMutation.error and shown in modal
    }
  }

  const mutationErrorMessage =
    createMutation.error instanceof ApiError
      ? createMutation.error.message
      : createMutation.error
        ? 'Failed to create collection.'
        : null

  return (
    <div className="flex flex-col gap-xl p-3xl">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-md">
        <div>
          <h1 className="text-3xl font-medium tracking-1 text-text">Collections</h1>
          <p className="text-sm text-text-2 mt-4xs">
            Organize your books into custom reading lists.
          </p>
        </div>

        <Button variant="primary" onClick={handleOpenCreateModal}>
          + New collection
        </Button>
      </div>

      {/* Polite live region for screen readers */}
      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {!isPending && `${collections.length} collections loaded`}
      </div>

      {/* Main Content */}
      <div>
        {isPending ? (
          <Spinner label="Loading collections" className="m-3xl" />
        ) : error ? (
          <ErrorState
            title="Something went wrong loading collections"
            description={error instanceof ApiError ? error.message : undefined}
            code={error instanceof ApiError ? error.code : undefined}
            correlationId={error instanceof ApiError ? error.correlationId : undefined}
            onRetry={() => void refetch()}
          />
        ) : collections.length === 0 ? (
          <EmptyState
            mascotMood="sleeping"
            title="No collections yet"
            description="Create a collection to organize your library into custom reading lists."
            action={{
              label: 'New collection',
              onClick: handleOpenCreateModal,
            }}
          />
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-lg">
            {collections.map((c: CollectionSummary) => (
              <Link
                key={c.id}
                to={`/collections/${c.id}`}
                className={cx(
                  'group flex flex-col justify-between rounded-lg border border-border bg-surface p-lg transition-all hover:border-text-3 hover:bg-surface-2 hover:shadow-sm',
                  FOCUS_RING,
                )}
              >
                <div className="flex flex-col gap-xs">
                  <div className="flex items-center gap-sm">
                    <div className="size-9 rounded-md bg-surface-2 border border-border text-text-2 flex items-center justify-center shrink-0 group-hover:border-text-3 group-hover:text-text transition-colors">
                      <FolderIcon className="size-5" />
                    </div>
                    <h2 className="text-lg font-medium text-text truncate">
                      {c.name}
                    </h2>
                  </div>
                </div>

                <div className="mt-md flex items-center justify-between border-t border-border/50 pt-sm">
                  <span className="text-xs text-text-2 font-ui">
                    {c.workCount} {c.workCount === 1 ? 'book' : 'books'}
                  </span>
                  <div className="flex items-center gap-4xs text-xs text-text-3 group-hover:text-text group-hover:translate-x-0.5 transition-all">
                    <span>View</span>
                    <ChevronRightIcon className="size-3.5" />
                  </div>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>

      {/* Create Collection Modal */}
      <Modal
        open={isCreateOpen}
        onOpenChange={setIsCreateOpen}
        title="New collection"
        description="Enter a name for your new collection."
      >
        <form onSubmit={handleCreateSubmit} className="mt-md flex flex-col gap-md">
          <Input
            ref={nameInputRef}
            label="Collection name"
            value={newCollectionName}
            onChange={(e) => {
              setNewCollectionName(e.target.value)
              if (validationError) setValidationError(null)
            }}
            placeholder="e.g. Favorites, Science Fiction, Research"
            maxLength={100}
            error={validationError ?? undefined}
          />

          {mutationErrorMessage && (
            <p className="text-xs text-error font-ui" role="alert">
              {mutationErrorMessage}
            </p>
          )}

          <div className="mt-sm flex items-center justify-end gap-sm">
            <Button variant="ghost" type="button" onClick={() => setIsCreateOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              type="submit"
              disabled={createMutation.isPending || !newCollectionName.trim()}
            >
              {createMutation.isPending ? 'Creating...' : 'Create collection'}
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}
