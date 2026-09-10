import { useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Button } from '../../components/Button/Button'
import { EmptyState } from '../../components/EmptyState/EmptyState'
import { ErrorState } from '../../components/ErrorState/ErrorState'
import { FormatBadge } from '../../components/FormatBadge/FormatBadge'
import { GeneratedCover } from '../../components/GeneratedCover/GeneratedCover'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  type ImportCandidate,
  type MatchCandidate,
  useConfirmImportCandidate,
  useImportCandidates,
  useRejectImportCandidate,
} from '../../data/import'
import { ApiError } from '../../data/http'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import { coverImageSrc } from '../../lib/imageDataUri'

const DISMISSED_STORAGE_KEY = 'alexandryn_dismissed_import_failures'

function getDismissedIds(): string[] {
  try {
    const raw = localStorage.getItem(DISMISSED_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function addDismissedId(id: string) {
  try {
    const current = getDismissedIds()
    if (!current.includes(id)) {
      current.push(id)
      localStorage.setItem(DISMISSED_STORAGE_KEY, JSON.stringify(current))
    }
  } catch {
    // Ignore storage errors
  }
}

function friendlyFailureReason(lastError: string | null | undefined): string {
  if (!lastError) {
    return 'This file could not be processed.'
  }
  const err = lastError.toLowerCase()
  if (err.includes('notitle') || err.includes('no title')) {
    return 'Could not find a title in this file’s metadata.'
  }
  if (err.includes('oversized') || err.includes('size')) {
    return 'This file exceeds the maximum allowed size.'
  }
  return 'This file appears to be damaged or is not a valid EPUB/PDF/CBZ file.'
}

function ConfidenceBadge({ confidence }: { confidence: 'exact' | 'high' | 'medium' | 'low' }) {
  const labelMap = {
    exact: 'Exact match',
    high: 'High confidence',
    medium: 'Medium confidence',
    low: 'Low confidence',
  }
  const colorMap = {
    exact: 'border-accent-2/40 bg-accent-2/10 text-accent-2',
    high: 'border-accent/40 bg-accent/10 text-accent',
    medium: 'border-warning/40 bg-warning/10 text-warning',
    low: 'border-border-2 bg-surface-2 text-text-2',
  }

  return (
    <span
      className={cx(
        'inline-flex items-center rounded-3xs border px-2xs py-4xs text-3xs font-medium uppercase tracking-wider',
        colorMap[confidence] ?? colorMap.low,
      )}
    >
      {labelMap[confidence] ?? confidence}
    </span>
  )
}

function CandidateCover({
  candidate,
  title,
  author,
}: {
  candidate: ImportCandidate
  title: string
  author?: string
}) {
  const [imgFailed, setImgFailed] = useState(false)
  // Extracted from a book file the source provided — reject a data: URI
  // that is not an allowed raster image type (audit 0016 #171).
  const inlineSrc = coverImageSrc(candidate.extractedMetadata?.coverBytes)
  // Serve extracted covers at a dedicated URL (audit 0016 #171), falling back to inline cover or generated cover
  const coverUrl = `/api/v1/import/candidates/${encodeURIComponent(candidate.id)}/cover`
  const src = inlineSrc || (candidate.extractedMetadata?.coverBytes ? null : (candidate.id ? coverUrl : null))

  if (src && !imgFailed) {
    return (
      <img
        src={src}
        alt={`Cover for ${title}`}
        loading="lazy"
        onError={() => setImgFailed(true)}
        className="size-full rounded-xs object-cover shadow-sm aspect-[2/3]"
      />
    )
  }

  return (
    <GeneratedCover
      identifier={candidate.id}
      title={title}
      author={author}
      className="size-full"
    />
  )
}

function CandidateCard({
  candidate,
  onConfirmed,
  onRejected,
}: {
  candidate: ImportCandidate
  onConfirmed?: () => void
  onRejected?: () => void
}) {
  const [inlineError, setInlineError] = useState<string | null>(null)
  const confirmMutation = useConfirmImportCandidate()
  const rejectMutation = useRejectImportCandidate()

  const meta = candidate.extractedMetadata
  const title = meta?.title || 'Untitled'
  const author = meta?.authors && meta.authors.length > 0 ? meta.authors.join(', ') : 'Unknown author'
  const matches = candidate.matchCandidates || []

  const isPending = confirmMutation.isPending || rejectMutation.isPending

  const handleConfirmMatch = (match: MatchCandidate) => {
    setInlineError(null)
    const isExisting = match.type === 'existing_edition'
    confirmMutation.mutate(
      {
        id: candidate.id,
        payload: {
          action: isExisting ? 'attach_existing' : 'use_open_library_match',
          editionId: match.editionId,
          openLibraryWorkKey: match.openLibraryWorkKey,
        },
      },
      {
        onSuccess: () => {
          onConfirmed?.()
        },
        onError: (err) => {
          setInlineError(err instanceof ApiError ? err.message : 'Confirmation failed')
        },
      },
    )
  }

  const handleCreateNew = () => {
    setInlineError(null)
    confirmMutation.mutate(
      {
        id: candidate.id,
        payload: {
          action: 'create_new',
        },
      },
      {
        onSuccess: () => {
          onConfirmed?.()
        },
        onError: (err) => {
          setInlineError(err instanceof ApiError ? err.message : 'Adding book failed')
        },
      },
    )
  }

  const handleReject = () => {
    setInlineError(null)
    rejectMutation.mutate(candidate.id, {
      onSuccess: () => {
        onRejected?.()
      },
      onError: (err) => {
        setInlineError(err instanceof ApiError ? err.message : 'Rejecting candidate failed')
      },
    })
  }

  return (
    <article
      className="flex flex-col gap-md rounded-md border border-border-1 bg-surface-1 p-lg shadow-sm"
      aria-label={`Import candidate: ${title}`}
    >
      <div className="flex items-center justify-between border-b border-border-1 pb-xs">
        <div className="flex items-center gap-xs">
          <FormatBadge format={candidate.fileReference.format} />
          <span className="text-xs font-mono text-text-3 truncate max-w-xs" title={candidate.fileReference.id}>
            {candidate.fileReference.id}
          </span>
        </div>
        <Button
          variant="secondary"
          onClick={handleReject}
          disabled={isPending}
          className="text-xs py-4xs px-xs"
        >
          Reject
        </Button>
      </div>

      {inlineError && (
        <div
          className="rounded-xs border border-danger/30 bg-danger/10 p-xs text-xs text-danger"
          role="alert"
        >
          {inlineError}
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-4 gap-md">
        {/* Extracted book column */}
        <div className="flex gap-md md:col-span-2">
          <div className="w-24 shrink-0">
            <CandidateCover candidate={candidate} title={title} author={author} />
          </div>
          <div className="flex flex-col gap-2xs min-w-0">
            <span className="text-3xs uppercase tracking-wider text-text-3 font-medium">Extracted from file</span>
            <h3 className="text-base font-semibold text-text truncate" title={title}>
              {title}
            </h3>
            <p className="text-xs text-text-2 truncate" title={author}>
              {author}
            </p>
            {meta?.isbn && (
              <p className="text-3xs font-mono text-text-3">ISBN: {meta.isbn}</p>
            )}
            {meta?.publisher && (
              <p className="text-3xs text-text-3 truncate">Publisher: {meta.publisher}</p>
            )}
          </div>
        </div>

        {/* Matches & Actions column */}
        <div className="flex flex-col gap-sm md:col-span-2 border-t md:border-t-0 md:border-l border-border-1 pt-sm md:pt-0 md:pl-md">
          {matches.length > 0 ? (
            <div className="flex flex-col gap-xs">
              <span className="text-3xs uppercase tracking-wider text-text-3 font-medium">
                Suggested matches
              </span>
              {matches.map((match, idx) => (
                <div
                  key={idx}
                  className="flex items-center justify-between gap-xs rounded-xs border border-border-2 bg-surface-2 p-xs"
                >
                  <div className="flex items-center gap-xs min-w-0">
                    {match.coverUrl ? (
                      <img
                        src={match.coverUrl}
                        alt=""
                        className="size-8 rounded-2xs object-cover shrink-0"
                      />
                    ) : (
                      <div className="size-8 rounded-2xs bg-surface-3 shrink-0 flex items-center justify-center text-xs">
                        📖
                      </div>
                    )}
                    <div className="flex flex-col min-w-0">
                      <div className="flex items-center gap-2xs">
                        <ConfidenceBadge confidence={match.confidence} />
                      </div>
                      <span className="text-xs font-medium text-text truncate" title={match.title}>
                        {match.title || title}
                      </span>
                      {match.author && (
                        <span className="text-3xs text-text-2 truncate">{match.author}</span>
                      )}
                    </div>
                  </div>
                  <Button
                    variant="primary"
                    onClick={() => handleConfirmMatch(match)}
                    disabled={isPending}
                    className="text-xs shrink-0 py-4xs px-xs"
                  >
                    Use this match
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-xs text-text-3 italic">No bibliographic matches found.</p>
          )}

          <div className="pt-xs border-t border-border-1 flex justify-end">
            <Button
              variant="secondary"
              onClick={handleCreateNew}
              disabled={isPending}
              className="text-xs py-4xs px-xs"
            >
              Add from extracted info
            </Button>
          </div>
        </div>
      </div>
    </article>
  )
}

/**
 * Import confirmation screen at /import (frontend-import-confirmation.md FR-2).
 */
export function Import() {
  const [searchParams] = useSearchParams()
  const sourceIdParam = searchParams.get('sourceId') || undefined

  const [dismissedIds, setDismissedIds] = useState<string[]>(() => getDismissedIds())

  // In-flight progress query (FR-6): adaptive polling — 2s while items
  // are queued, 20s when the queue is empty, off while the tab is hidden
  // (audit 0016 #169).
  const { data: queuedData } = useImportCandidates({
    sourceId: sourceIdParam,
    status: 'queued',
    refetchInterval: true,
  })
  const queuedCount = queuedData?.candidates.length ?? 0

  // Pending candidates query (FR-2)
  const {
    data: pendingData,
    error: pendingError,
    isPending: isPendingLoading,
    refetch: refetchPending,
  } = useImportCandidates({
    sourceId: sourceIdParam,
    status: 'pending',
  })

  // Failed candidates query (FR-5)
  const { data: failedData, refetch: refetchFailed } = useImportCandidates({
    sourceId: sourceIdParam,
    status: 'failed',
  })

  const pendingCandidates = pendingData?.candidates ?? []
  const failedCandidates = (failedData?.candidates ?? []).filter(
    (c) => !dismissedIds.includes(c.id),
  )

  const handleDismissFailed = (id: string) => {
    addDismissedId(id)
    setDismissedIds(getDismissedIds())
  }

  return (
    <div className="flex flex-col gap-xl p-3xl max-w-5xl">
      {/* Header */}
      <div className="flex flex-col gap-sm">
        <div className="flex flex-wrap items-center justify-between gap-md">
          <div>
            <h1 className="text-3xl font-medium tracking-1 text-text">Import review</h1>
            <p className="text-sm text-text-2 mt-4xs">
              Review extracted metadata and resolve suggested bibliographic matches for new books.
            </p>
          </div>
          {sourceIdParam && (
            <Link
              to={`/sources/${encodeURIComponent(sourceIdParam)}`}
              className={cx(
                'text-xs font-medium text-text-2 hover:text-text rounded py-4xs px-xs border border-border-1 bg-surface-1',
                FOCUS_RING,
              )}
            >
              ← Back to source
            </Link>
          )}
        </div>
      </div>

      {/* In-flight processing banner (FR-6) */}
      {queuedCount > 0 && (
        <div
          className="flex items-center gap-sm rounded-md border border-accent/30 bg-accent/10 p-md text-accent text-sm font-medium"
          role="status"
          aria-live="polite"
        >
          <Spinner label="Processing files" className="size-4" />
          <span>
            Processing {queuedCount} file{queuedCount === 1 ? '' : 's'} in the background...
          </span>
        </div>
      )}

      {/* Pending section (FR-2) */}
      {isPendingLoading ? (
        <Spinner label="Loading pending import candidates" className="my-3xl" />
      ) : pendingError ? (
        <ErrorState
          title="Could not load pending candidates"
          description={
            pendingError instanceof ApiError
              ? pendingError.message
              : 'The server could not be reached. Check your connection and retry.'
          }
          onRetry={() => void refetchPending()}
        />
      ) : pendingCandidates.length === 0 ? (
        <EmptyState
          title="No pending imports"
          description="All discovered books have been reviewed and imported."
        />
      ) : (
        <div className="flex flex-col gap-lg">
          <h2 className="text-lg font-medium text-text">
            Needs review ({pendingCandidates.length})
          </h2>
          <div className="flex flex-col gap-md">
            {pendingCandidates.map((cand) => (
              <CandidateCard
                key={cand.id}
                candidate={cand}
                onConfirmed={() => {
                  void refetchPending()
                  void refetchFailed()
                }}
                onRejected={() => {
                  void refetchPending()
                }}
              />
            ))}
          </div>
        </div>
      )}

      {/* Failed section (FR-5) */}
      {failedCandidates.length > 0 && (
        <div className="flex flex-col gap-md border-t border-border-1 pt-xl mt-md">
          <div>
            <h2 className="text-lg font-medium text-danger">
              Couldn’t be imported ({failedCandidates.length})
            </h2>
            <p className="text-xs text-text-3 mt-4xs">
              These files could not be extracted or read. You can dismiss them from this review list.
            </p>
          </div>

          <div className="flex flex-col gap-sm">
            {failedCandidates.map((cand) => (
              <div
                key={cand.id}
                className="flex items-center justify-between gap-md rounded-md border border-danger/20 bg-danger/5 p-md"
              >
                <div className="flex items-center gap-sm min-w-0">
                  <FormatBadge format={cand.fileReference.format} />
                  <div className="flex flex-col min-w-0">
                    <span className="text-xs font-mono text-text truncate" title={cand.fileReference.id}>
                      {cand.fileReference.id}
                    </span>
                    <span className="text-3xs text-danger font-medium">
                      {friendlyFailureReason(cand.lastError)}
                    </span>
                  </div>
                </div>
                <Button
                  variant="secondary"
                  onClick={() => handleDismissFailed(cand.id)}
                  className="text-xs py-4xs px-xs shrink-0"
                >
                  Dismiss
                </Button>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
