import { useState, type FormEvent } from 'react'
import { Button } from '../../components/Button/Button'
import { Input } from '../../components/Input/Input'
import { Modal } from '../../components/Modal/Modal'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  addWorkToCollection,
  createCollection,
  removeWorkFromCollection,
  useCollections,
  type CollectionDetail,
} from '../../data/collections'
import { ApiError } from '../../data/http'
import type { WorkDetail } from '../../data/library'
import { useQueryClient } from '@tanstack/react-query'

export interface AddToCollectionModalProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  work: WorkDetail
}

type ModalState =
  | { type: 'idle' }
  | { type: 'creating_and_adding' }
  | { type: 'create_failed'; error: string }
  | {
      type: 'partial_failure'
      createdCollection: CollectionDetail
      error: string
    }
  | { type: 'ambiguous_outcome'; message: string }

export function AddToCollectionModal({
  open,
  onOpenChange,
  work,
}: AddToCollectionModalProps) {
  const queryClient = useQueryClient()
  const { data: collections = [], isPending: isLoadingCollections } = useCollections()

  const [modalState, setModalState] = useState<ModalState>({ type: 'idle' })
  const [newCollectionName, setNewCollectionName] = useState('')
  const [togglingIds, setTogglingIds] = useState<Set<string>>(new Set())

  // Check if work is already in a collection
  const memberCollectionIds = new Set(work.collections?.map((c) => c.id) ?? [])

  const handleToggleMembership = async (collectionId: string, isCurrentlyMember: boolean) => {
    setTogglingIds((prev) => new Set(prev).add(collectionId))
    try {
      if (isCurrentlyMember) {
        await removeWorkFromCollection(collectionId, work.id)
      } else {
        await addWorkToCollection(collectionId, work.id)
      }
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['collection', collectionId] })
      void queryClient.invalidateQueries({ queryKey: ['work', work.id] })
    } catch {
      // Ignore or surface toggle error
    } finally {
      setTogglingIds((prev) => {
        const next = new Set(prev)
        next.delete(collectionId)
        return next
      })
    }
  }

  const handleCreateAndAddSubmit = async (e: FormEvent) => {
    e.preventDefault()
    const trimmed = newCollectionName.trim()
    if (!trimmed) return

    setModalState({ type: 'creating_and_adding' })

    let newCollection: CollectionDetail | null = null

    // Step 1: Create collection
    try {
      newCollection = await createCollection(trimmed)
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
    } catch (err: unknown) {
      const isAbort = (err as { name?: string })?.name === 'AbortError'
      if (isAbort) {
        // Case (c): Ambiguous outcome — request aborted / timeout
        setModalState({
          type: 'ambiguous_outcome',
          message: 'Unable to confirm — check your collections list.',
        })
        return
      }

      // Case (a): Create failed outright

      const message =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : 'Failed to create collection.'
      setModalState({ type: 'create_failed', error: message })
      return
    }

    // Step 2: Add work to the newly created collection
    try {
      await addWorkToCollection(newCollection.id, work.id)
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['collection', newCollection.id] })
      void queryClient.invalidateQueries({ queryKey: ['work', work.id] })

      // Full success: reset and close
      setNewCollectionName('')
      setModalState({ type: 'idle' })
      onOpenChange(false)
    } catch (err: unknown) {
      // Case (b): Partial failure — create succeeded, add failed
      const message =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : 'Failed to add book to new collection.'
      setModalState({
        type: 'partial_failure',
        createdCollection: newCollection,
        error: message,
      })
    }
  }

  const handleRetryAdd = async (createdCollection: CollectionDetail) => {
    setModalState({ type: 'creating_and_adding' })
    try {
      await addWorkToCollection(createdCollection.id, work.id)
      void queryClient.invalidateQueries({ queryKey: ['collections'] })
      void queryClient.invalidateQueries({ queryKey: ['collection', createdCollection.id] })
      void queryClient.invalidateQueries({ queryKey: ['work', work.id] })

      setNewCollectionName('')
      setModalState({ type: 'idle' })
      onOpenChange(false)
    } catch (err: unknown) {
      const message =
        err instanceof ApiError
          ? err.message
          : err instanceof Error
            ? err.message
            : 'Failed to add book to collection.'
      setModalState({
        type: 'partial_failure',
        createdCollection,
        error: message,
      })
    }
  }

  const handleModalClose = (open: boolean) => {
    if (!open) {
      setModalState({ type: 'idle' })
    }
    onOpenChange(open)
  }

  return (
    <Modal
      open={open}
      onOpenChange={handleModalClose}
      title="Add to collection"
      description={`Manage collection memberships for "${work.title}".`}
    >
      <div className="mt-md flex flex-col gap-lg">
        {/* Ambiguous outcome warning (Case c) */}
        {modalState.type === 'ambiguous_outcome' && (
          <div className="rounded-xs border border-warning/40 bg-warning/10 p-sm text-xs text-text">
            <p className="font-medium text-warning">{modalState.message}</p>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleModalClose(false)}
              className="mt-xs"
            >
              Close
            </Button>
          </div>
        )}

        {/* Partial failure state (Case b) */}
        {modalState.type === 'partial_failure' && (
          <div className="rounded-xs border border-warning/40 bg-warning/10 p-sm text-xs text-text flex flex-col gap-xs">
            <p>
              Collection <strong>&quot;{modalState.createdCollection.name}&quot;</strong> was created,
              but adding this book failed.
            </p>
            <p className="text-text-2">{modalState.error}</p>
            <div className="mt-2xs flex items-center gap-xs">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => void handleRetryAdd(modalState.createdCollection)}
              >
                Try adding again
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setModalState({ type: 'idle' })
                  setNewCollectionName('')
                }}
              >
                Dismiss
              </Button>
            </div>
          </div>
        )}

        {/* Existing Collections List */}
        <div className="flex flex-col gap-xs">
          <span className="text-xs font-medium text-text-2 uppercase tracking-wide">
            Your collections
          </span>

          {isLoadingCollections ? (
            <Spinner label="Loading collections" className="my-sm" />
          ) : collections.length === 0 ? (

            <p className="text-xs text-text-3 py-xs">No collections created yet.</p>
          ) : (
            <div className="max-h-48 overflow-y-auto flex flex-col gap-2xs pr-xs">
              {collections.map((col) => {
                const isMember = memberCollectionIds.has(col.id)
                const isToggling = togglingIds.has(col.id)

                return (
                  <label
                    key={col.id}
                    className="flex items-center justify-between gap-sm rounded-xs border border-border bg-surface-2 px-sm py-2xs text-sm cursor-pointer hover:bg-surface-3 transition-colors"
                  >
                    <div className="flex items-center gap-xs min-w-0">
                      <input
                        type="checkbox"
                        checked={isMember}
                        disabled={isToggling}
                        onChange={() => void handleToggleMembership(col.id, isMember)}
                        className="rounded-3xs accent-accent size-4 cursor-pointer"
                      />
                      <span className="truncate text-text font-ui">{col.name}</span>
                    </div>
                    <span className="text-xs text-text-3 shrink-0">
                      {col.workCount} {col.workCount === 1 ? 'book' : 'books'}
                    </span>
                  </label>
                )
              })}
            </div>
          )}
        </div>

        {/* Create New Collection and Add (FR-4) */}
        <form
          onSubmit={handleCreateAndAddSubmit}
          className="border-t border-border/60 pt-md flex flex-col gap-sm"
        >
          <span className="text-xs font-medium text-text-2 uppercase tracking-wide">
            Create new collection and add
          </span>

          <div className="flex items-end gap-sm">
            <div className="flex-1">
              <Input
                label="New collection name"
                value={newCollectionName}
                onChange={(e) => {
                  setNewCollectionName(e.target.value)
                  if (modalState.type === 'create_failed') {
                    setModalState({ type: 'idle' })
                  }
                }}
                placeholder="e.g. Must Read, Essays"
                maxLength={100}
                error={
                  modalState.type === 'create_failed' ? modalState.error : undefined
                }
              />
            </div>

            <Button
              variant="primary"
              type="submit"
              disabled={
                modalState.type === 'creating_and_adding' || !newCollectionName.trim()
              }
            >
              {modalState.type === 'creating_and_adding' ? 'Creating...' : 'Create & Add'}
            </Button>
          </div>
        </form>

        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={() => handleModalClose(false)}>
            Done
          </Button>
        </div>
      </div>
    </Modal>
  )
}
