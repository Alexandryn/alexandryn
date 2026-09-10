import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import {
  parseActivityEvents,
  useActivityEvents,
  useCancelJob,
  useClearCompleted,
  usePauseAll,
  useRetryJob,
  type ActivityItem,
} from '../../data/activity'
import { Button } from '../../components/Button'
import { Modal } from '../../components/Modal'
import { FOCUS_RING } from '../../lib/focusRing'
import { cx } from '../../lib/cx'

export function ActivityScreen() {
  const { data: events = [], isLoading, isError, refetch } = useActivityEvents({ active: true })
  const pauseAll = usePauseAll()
  const cancelJob = useCancelJob()
  const retryJob = useRetryJob()
  const clearCompleted = useClearCompleted()

  // Cancelling an in-progress download cannot be undone, so it goes
  // through an explicit confirmation rather than firing on the first
  // click (audit 0016 #300).
  const [pendingCancel, setPendingCancel] = useState<ActivityItem | null>(null)

  // Memoize grouped events to prevent downstream memo breakage across poll ticks (audit 0016 #214).
  const grouped = useMemo(() => parseActivityEvents(events), [events])
  const isEmpty =
    grouped.active.length === 0 &&
    grouped.queued.length === 0 &&
    grouped.failed.length === 0 &&
    grouped.completed.length === 0

  return (
    <div className="p-[30px_clamp(20px,2.6vw,40px)_80px] max-w-[68.75rem] w-full">
      {/* Header */}
      <h1 className="m-0 text-[1.6875rem] font-medium tracking-1 text-text">Activity</h1>
      <div className="text-2xl text-text-2 mt-1.5 mb-6">
        What Alexandryn is fetching from your sources, and what you have been reading.
      </div>

      {/* Screen reader live announcements */}
      <div aria-live="polite" className="sr-only">
        {grouped.active.length > 0 && `${grouped.active.length} active downloads`}
        {grouped.failed.length > 0 && `, ${grouped.failed.length} failed items`}
      </div>

      {isError && (
        <div className="mb-6 p-4 rounded-lg border border-error bg-surface flex items-center justify-between">
          <span className="text-sm text-error">Failed to load activity feed.</span>
          <button
            type="button"
            onClick={() => refetch()}
            className={cx('text-xs text-text font-medium underline', FOCUS_RING)}
          >
            Retry connection
          </button>
        </div>
      )}

      {isLoading && isEmpty ? (
        <div className="py-12 text-center text-text-3 text-sm">Loading activity...</div>
      ) : isEmpty ? (
        <div className="py-12 text-center text-text-3 text-sm">
          No recent activity. Books you import or acquire from sources will appear here.
        </div>
      ) : (
        <>
          {/* ACTIVE section */}
          {grouped.active.length > 0 && (
            <section aria-labelledby="active-heading" className="cv-auto mb-[1.875rem]">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="active-heading"
                  className="font-mono text-3xs tracking-9 text-text-3 font-normal uppercase"
                >
                  ACTIVE
                </h2>
                <div className="flex-1 h-px bg-border" />
                <button
                  type="button"
                  onClick={() => pauseAll.mutate()}
                  disabled={pauseAll.isPending}
                  className={cx('text-xs text-text-2 hover:text-text cursor-pointer', FOCUS_RING)}
                >
                  Pause all
                </button>
              </div>

              <div className="flex flex-col gap-2">
                {grouped.active.map((item) => (
                  <ActiveRow key={item.id} item={item} onCancel={() => setPendingCancel(item)} />
                ))}
              </div>
            </section>
          )}

          {/* QUEUED section */}
          {grouped.queued.length > 0 && (
            <section aria-labelledby="queued-heading" className="cv-auto mb-[1.875rem]">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="queued-heading"
                  className="font-mono text-3xs tracking-9 text-text-3 font-normal uppercase"
                >
                  QUEUED
                </h2>
                <div className="flex-1 h-px bg-border" />
              </div>

              <div className="border border-border rounded-lg bg-surface overflow-hidden divide-y divide-border-2">
                {grouped.queued.map((item) => (
                  <QueuedRow
                    key={item.id}
                    item={item}
                    onCancel={() => {
                      if (item.jobId) cancelJob.mutate(item.jobId)
                    }}
                  />
                ))}
              </div>
            </section>
          )}

          {/* FAILED section */}
          {grouped.failed.length > 0 && (
            <section aria-labelledby="failed-heading" className="cv-auto mb-[1.875rem]">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="failed-heading"
                  className="font-mono text-3xs tracking-9 text-text-3 font-normal uppercase"
                >
                  FAILED
                </h2>
                <div className="flex-1 h-px bg-border" />
              </div>

              <div className="flex flex-col gap-2">
                {grouped.failed.map((item) => (
                  <FailedRow
                    key={item.id}
                    item={item}
                    onRetry={() => {
                      if (item.jobId) retryJob.mutate(item.jobId)
                    }}
                  />
                ))}
              </div>
            </section>
          )}

          {/* COMPLETED section */}
          {grouped.completed.length > 0 && (
            <section aria-labelledby="completed-heading" className="cv-auto">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="completed-heading"
                  className="font-mono text-3xs tracking-9 text-text-3 font-normal uppercase"
                >
                  COMPLETED
                </h2>
                <div className="flex-1 h-px bg-border" />
                <button
                  type="button"
                  onClick={() => clearCompleted.mutate()}
                  disabled={clearCompleted.isPending}
                  className={cx('text-xs text-text-2 hover:text-text cursor-pointer', FOCUS_RING)}
                >
                  Clear
                </button>
              </div>

              <div className="border border-border rounded-lg bg-surface overflow-hidden divide-y divide-border-2">
                {grouped.completed.map((item) => (
                  <CompletedRow key={item.id} item={item} />
                ))}
              </div>
            </section>
          )}
        </>
      )}

      <Modal
        open={pendingCancel !== null}
        onOpenChange={(open) => {
          if (!open) setPendingCancel(null)
        }}
        title="Cancel this download?"
        description={
          pendingCancel
            ? `"${pendingCancel.title}" will stop downloading. Progress so far is discarded and you will need to start it again.`
            : undefined
        }
        contentClassName="max-w-md"
      >
        <div className="mt-md flex items-center justify-end gap-sm pt-sm border-t border-border">
          <Button variant="ghost" size="sm" onClick={() => setPendingCancel(null)}>
            Keep downloading
          </Button>
          <Button
            variant="primary"
            size="sm"
            className="bg-error hover:bg-error/90 text-white"
            onClick={() => {
              if (pendingCancel?.jobId) cancelJob.mutate(pendingCancel.jobId)
              setPendingCancel(null)
            }}
            disabled={cancelJob.isPending}
          >
            Cancel download
          </Button>
        </div>
      </Modal>
    </div>
  )
}

function ActiveRow({ item, onCancel }: { item: ActivityItem; onCancel: () => void }) {
  const pct = item.progressPercent ?? 0
  return (
    <div className="flex items-center gap-4 p-3.5 px-4 rounded-lg border border-border bg-surface shadow-sm">
      <div className="relative w-[2.125rem] flex-none aspect-[2/3] rounded-5xs overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-5xs text-text-3">
        <div className="absolute left-0 top-0 bottom-0 w-0.5 bg-black/25" />
        {(item.format || 'EPB').slice(0, 3)}
      </div>

      <div className="w-[16.25rem] flex-[1_1_200px] min-w-0">
        <div className="text-2xl font-medium truncate text-text">{item.title}</div>
        <div className="text-xs text-text-3 truncate mt-0.75">
          {item.author || 'Unknown author'}
        </div>
      </div>

      <div className="w-[10.625rem] flex-[0_1_170px] min-w-0 font-mono text-2xs text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-none px-2 py-0.75 rounded-4xs bg-surface-3 font-mono text-4xs text-text-2">
        {item.format}
      </div>

      <div className="flex-[1_1_160px] min-w-0">
        <div className="h-0.75 rounded-5xs bg-border overflow-hidden">
          <div
            className="h-full rounded-5xs bg-accent transition-all duration-300"
            style={{ width: `${Math.min(Math.max(pct, 0), 100)}%` }}
          />
        </div>
        <div className="font-mono text-3xs text-text-3 mt-1.5 truncate">
          {item.progressText || `${pct}%`}
        </div>
      </div>

      <button
        type="button"
        onClick={onCancel}
        aria-label={`Cancel ${item.title}`}
        className={cx(
          'flex-none inline-flex items-center h-7 px-3 rounded-2xs border border-border text-xs text-text-2 hover:text-error cursor-pointer bg-surface',
          FOCUS_RING,
        )}
      >
        Cancel
      </button>
    </div>
  )
}

function QueuedRow({ item, onCancel }: { item: ActivityItem; onCancel: () => void }) {
  return (
    <div className="flex items-center gap-4 py-2.5 px-4">
      <div className="relative w-[1.625rem] flex-none aspect-[2/3] rounded-5xs overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-[0.5rem] text-text-3">
        {(item.format || 'EPB').slice(0, 3)}
      </div>

      <div className="flex-[0_1_260px] min-w-0 text-xl truncate text-text">{item.title}</div>

      <div className="flex-[0_1_170px] min-w-0 font-mono text-2xs text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-none px-2 py-0.75 rounded-4xs bg-surface-3 font-mono text-4xs text-text-2">
        {item.format}
      </div>

      <div className="flex-1 min-w-0 text-xs text-text-3 truncate">
        {item.progressText || 'Waiting for source'}
      </div>

      <button
        type="button"
        onClick={onCancel}
        className={cx('flex-none text-xs text-text-2 hover:text-error cursor-pointer', FOCUS_RING)}
      >
        Cancel
      </button>
    </div>
  )
}

function FailedRow({ item, onRetry }: { item: ActivityItem; onRetry: () => void }) {
  return (
    <div className="flex items-center gap-4 p-3.5 px-4 rounded-lg border border-border bg-surface">
      <div className="relative w-[2.125rem] flex-none aspect-[2/3] rounded-5xs overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-5xs text-text-3">
        {(item.format || 'EPB').slice(0, 3)}
      </div>

      <div className="flex-[0_1_260px] min-w-0 text-2xl font-medium truncate text-text">
        {item.title}
      </div>

      <div className="flex-[0_1_170px] min-w-0 font-mono text-2xs text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-1 min-w-0 flex items-center gap-2">
        <div className="w-1.25 h-1.25 rounded-full bg-error flex-none" />
        <div className="text-lg text-error truncate">{item.errorMessage || 'Operation failed'}</div>
      </div>

      <Link
        to="/sources"
        className={cx(
          'text-xs text-text-2 hover:text-text cursor-pointer underline-offset-2 hover:underline',
          FOCUS_RING,
        )}
      >
        Fix source
      </Link>

      <button
        type="button"
        onClick={onRetry}
        className={cx(
          'inline-flex items-center h-7 px-3 rounded-2xs border border-border text-xs text-text-2 hover:text-text cursor-pointer bg-surface',
          FOCUS_RING,
        )}
      >
        Retry
      </button>
    </div>
  )
}

function CompletedRow({ item }: { item: ActivityItem }) {
  const destination = item.workId ? `/book/${item.workId}` : '/library'
  return (
    <div className="flex items-center gap-4 py-2.5 px-4">
      <div className="relative w-[1.625rem] flex-none aspect-[2/3] rounded-5xs overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-[0.5rem] text-text-3">
        {(item.format || 'EPB').slice(0, 3)}
      </div>

      <div className="flex-[0_1_260px] min-w-0 text-xl truncate text-text">{item.title}</div>

      <div className="flex-[0_1_170px] min-w-0 font-mono text-2xs text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-none px-2 py-0.75 rounded-4xs bg-surface-3 font-mono text-4xs text-text-2">
        {item.format}
      </div>

      <div className="flex-1 min-w-0 flex items-center gap-2">
        <div className="w-1.25 h-1.25 rounded-full bg-success flex-none" />
        <div className="text-lg text-text-2 truncate">Added to library</div>
      </div>

      <Link
        to={destination}
        className={cx('flex-none text-xs text-accent hover:underline font-medium', FOCUS_RING)}
      >
        Read
      </Link>
    </div>
  )
}
