import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Input } from '../../components/Input/Input'
import { Modal } from '../../components/Modal/Modal'
import { Spinner } from '../../components/Spinner/Spinner'
import { WorkGrid } from '../../components/WorkGrid'
import {
  useCollection,
  useDeleteCollection,
  useRenameCollection,
} from '../../data/collections'
import { ApiError } from '../../data/http'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'

/**
 * Collection detail screen at /collections/:id and /collection/:id (FR-2, FR-3).
 * Renders collection name with inline rename, delete confirmation modal,
 * and member works via shared <WorkGrid>.
 */
export function CollectionDetail() {
  const navigate = useNavigate()
  const params = useParams<{ id?: string; '*'?: string }>()
  const id =
    params.id ||
    (params['*']
      ? params['*'].replace(/^(collections|collection)\//, '').split('/')[0]
      : '') ||
    ''

  const { data: collection, error, isPending, refetch } = useCollection(id)
  const renameMutation = useRenameCollection()
  const deleteMutation = useDeleteCollection()

  // Rename modal state
  const [isRenameOpen, setIsRenameOpen] = useState(false)
  const [renameValue, setRenameValue] = useState('')
  const [renameValidationError, setRenameValidationError] = useState<string | null>(null)

  // Delete modal state
  const [isDeleteOpen, setIsDeleteOpen] = useState(false)

  // View mode
  const [view, setView] = useState<'grid' | 'list'>('grid')

  if (isPending) {
    return <Spinner label="Loading collection details" className="m-3xl" />
  }

  if (error) {
    const is404 = error instanceof ApiError && error.status === 404
    if (is404) {
      return (
        <div className="p-3xl">
          <EmptyState
            title="This collection doesn't exist."
            description="We couldn't find a collection matching that identifier."
            action={{
              label: 'Back to Collections',
              onClick: () => navigate('/collections'),
            }}
          />
        </div>
      )
    }

    return (
      <div className="p-3xl">
        <ErrorState
          title="Something went wrong loading this collection"
          description={error instanceof ApiError ? error.message : undefined}
          code={error instanceof ApiError ? error.code : undefined}
          correlationId={error instanceof ApiError ? error.correlationId : undefined}
          onRetry={() => void refetch()}
        />
      </div>
    )
  }

  if (!collection) return null

  const handleOpenRename = () => {
    setRenameValue(collection.name)
    setRenameValidationError(null)
    renameMutation.reset()
    setIsRenameOpen(true)
  }

  const handleRenameSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const trimmed = renameValue.trim()
    if (!trimmed) {
      setRenameValidationError('Collection name is required.')
      return
    }
    if (trimmed.length > 100) {
      setRenameValidationError('Collection name cannot exceed 100 characters.')
      return
    }

    setRenameValidationError(null)
    try {
      await renameMutation.mutateAsync({ id: collection.id, name: trimmed })
      setIsRenameOpen(false)
    } catch {
      // Error handled via mutation state in modal
    }
  }

  const handleDeleteConfirm = async () => {
    try {
      await deleteMutation.mutateAsync(collection.id)
      setIsDeleteOpen(false)
      navigate('/collections')
    } catch {
      // Error handled via mutation state in modal
    }
  }

  const renameErrorMessage =
    renameMutation.error instanceof ApiError
      ? renameMutation.error.message
      : renameMutation.error
        ? 'Failed to rename collection.'
        : null

  const deleteErrorMessage =
    deleteMutation.error instanceof ApiError
      ? deleteMutation.error.message
      : deleteMutation.error
        ? 'Failed to delete collection.'
        : null

  return (
    <div className="flex flex-col gap-xl p-3xl">
      {/* Back navigation */}
      <div>
        <Link
          to="/collections"
          className={cx(
            'inline-flex items-center gap-2xs text-sm font-medium text-text-2 hover:text-text rounded-2xs',
            FOCUS_RING,
          )}
        >
          ← Back to Collections
        </Link>
      </div>

      {/* Screen Header */}
      <div className="flex flex-wrap items-center justify-between gap-md border-b border-border/50 pb-lg">
        <div className="flex flex-col gap-4xs">
          <div className="flex items-center gap-sm">
            <span className="text-2xl" aria-hidden="true">
              📁
            </span>
            <h1 className="text-3xl font-medium tracking-1 text-text">{collection.name}</h1>
          </div>
          <p className="text-sm text-text-2">
            {collection.works.length} {collection.works.length === 1 ? 'book' : 'books'}
          </p>
        </div>

        <div className="flex items-center gap-sm">
          <Button variant="secondary" size="sm" onClick={handleOpenRename}>
            Rename
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setIsDeleteOpen(true)}
            className="text-error hover:bg-error/10"
          >
            Delete
          </Button>
          <div className="ml-md border-l border-border pl-md flex items-center gap-xs">
            <button
              type="button"
              onClick={() => setView('grid')}
              className={cx(
                'rounded-xs px-xs py-4xs text-xs font-ui transition-colors',
                view === 'grid' ? 'bg-surface-3 text-text font-medium' : 'text-text-2 hover:text-text',
                FOCUS_RING,
              )}
              aria-label="Grid view"
            >
              Grid
            </button>
            <button
              type="button"
              onClick={() => setView('list')}
              className={cx(
                'rounded-xs px-xs py-4xs text-xs font-ui transition-colors',
                view === 'list' ? 'bg-surface-3 text-text font-medium' : 'text-text-2 hover:text-text',
                FOCUS_RING,
              )}
              aria-label="List view"
            >
              List
            </button>
          </div>
        </div>
      </div>

      {/* Polite live announcement */}
      <div aria-live="polite" aria-atomic="true" className="sr-only">
        {`${collection.works.length} books in this collection`}
      </div>

      {/* Works Content Area */}
      <div>
        {collection.works.length === 0 ? (
          <EmptyState
            title="This collection is empty"
            description="Add books from your library to this collection."
            action={{
              label: 'Browse library',
              onClick: () => navigate('/library'),
            }}
          />
        ) : (
          <WorkGrid works={collection.works} view={view} />
        )}
      </div>

      {/* Rename Modal (FR-2) */}
      <Modal
        open={isRenameOpen}
        onOpenChange={setIsRenameOpen}
        title="Rename collection"
        description="Enter a new name for this collection."
      >
        <form onSubmit={handleRenameSubmit} className="mt-md flex flex-col gap-md">
          <Input
            label="New name"
            value={renameValue}
            onChange={(e) => {
              setRenameValue(e.target.value)
              if (renameValidationError) setRenameValidationError(null)
            }}
            maxLength={100}
            error={renameValidationError ?? undefined}
          />


          {renameErrorMessage && (
            <p className="text-xs text-error font-ui" role="alert">
              {renameErrorMessage}
            </p>
          )}

          <div className="mt-sm flex items-center justify-end gap-sm">
            <Button
              variant="ghost"
              type="button"
              onClick={() => setIsRenameOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              type="submit"
              disabled={renameMutation.isPending || !renameValue.trim()}
            >
              {renameMutation.isPending ? 'Saving...' : 'Save name'}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Delete Confirmation Modal (FR-3, Constitution §11) */}
      <Modal
        open={isDeleteOpen}
        onOpenChange={setIsDeleteOpen}
        title="Delete collection"
        description={`Delete "${collection.name}"? The books in it will stay in your library.`}
      >
        <div className="mt-md flex flex-col gap-md">
          {deleteErrorMessage && (
            <p className="text-xs text-error font-ui" role="alert">
              {deleteErrorMessage}
            </p>
          )}

          <div className="mt-sm flex items-center justify-end gap-sm">
            <Button
              variant="ghost"
              type="button"
              onClick={() => setIsDeleteOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="secondary"
              type="button"
              onClick={handleDeleteConfirm}
              disabled={deleteMutation.isPending}
              className="border-error/40 text-error hover:bg-error/10 hover:border-error"
            >
              {deleteMutation.isPending ? 'Deleting...' : 'Delete collection'}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
