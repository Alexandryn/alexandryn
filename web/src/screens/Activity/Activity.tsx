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
import { FOCUS_RING } from '../../lib/focusRing'
import { cx } from '../../lib/cx'

export function ActivityScreen() {
  const { data: events = [], isLoading, isError, refetch } = useActivityEvents()
  const pauseAll = usePauseAll()
  const cancelJob = useCancelJob()
  const retryJob = useRetryJob()
  const clearCompleted = useClearCompleted()

  const grouped = parseActivityEvents(events)
  const isEmpty =
    grouped.active.length === 0 &&
    grouped.queued.length === 0 &&
    grouped.failed.length === 0 &&
    grouped.completed.length === 0

  return (
    <div className="p-[30px_clamp(20px,2.6vw,40px)_80px] max-w-[1100px] w-full">
      {/* Header */}
      <h1 className="m-0 text-[27px] font-medium tracking-[-0.025em] text-text">Activity</h1>
      <div className="text-[13.5px] text-text-2 mt-1.5 mb-6">
        What Alexandryn is fetching from your sources, and what you have been reading.
      </div>

      {/* Screen reader live announcements */}
      <div aria-live="polite" className="sr-only">
        {grouped.active.length > 0 && `${grouped.active.length} active downloads`}
        {grouped.failed.length > 0 && `, ${grouped.failed.length} failed items`}
      </div>

      {isError && (
        <div className="mb-6 p-4 rounded-[11px] border border-error bg-surface flex items-center justify-between">
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
            <section aria-labelledby="active-heading" className="mb-[30px]">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="active-heading"
                  className="font-mono text-[10px] tracking-[0.12em] text-text-3 font-normal uppercase"
                >
                  ACTIVE
                </h2>
                <div className="flex-1 h-[1px] bg-border" />
                <button
                  type="button"
                  onClick={() => pauseAll.mutate()}
                  disabled={pauseAll.isPending}
                  className={cx(
                    'text-xs text-text-2 hover:text-text cursor-pointer',
                    FOCUS_RING,
                  )}
                >
                  Pause all
                </button>
              </div>

              <div className="flex flex-col gap-2">
                {grouped.active.map((item) => (
                  <ActiveRow
                    key={item.id}
                    item={item}
                    onPause={() => {
                      if (item.jobId) cancelJob.mutate(item.jobId)
                    }}
                  />
                ))}
              </div>
            </section>
          )}

          {/* QUEUED section */}
          {grouped.queued.length > 0 && (
            <section aria-labelledby="queued-heading" className="mb-[30px]">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="queued-heading"
                  className="font-mono text-[10px] tracking-[0.12em] text-text-3 font-normal uppercase"
                >
                  QUEUED
                </h2>
                <div className="flex-1 h-[1px] bg-border" />
              </div>

              <div className="border border-border rounded-[11px] bg-surface overflow-hidden divide-y divide-border-2">
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
            <section aria-labelledby="failed-heading" className="mb-[30px]">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="failed-heading"
                  className="font-mono text-[10px] tracking-[0.12em] text-text-3 font-normal uppercase"
                >
                  FAILED
                </h2>
                <div className="flex-1 h-[1px] bg-border" />
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
            <section aria-labelledby="completed-heading">
              <div className="flex items-center gap-2.5 mb-3">
                <h2
                  id="completed-heading"
                  className="font-mono text-[10px] tracking-[0.12em] text-text-3 font-normal uppercase"
                >
                  COMPLETED
                </h2>
                <div className="flex-1 h-[1px] bg-border" />
                <button
                  type="button"
                  onClick={() => clearCompleted.mutate()}
                  disabled={clearCompleted.isPending}
                  className={cx(
                    'text-xs text-text-2 hover:text-text cursor-pointer',
                    FOCUS_RING,
                  )}
                >
                  Clear
                </button>
              </div>

              <div className="border border-border rounded-[11px] bg-surface overflow-hidden divide-y divide-border-2">
                {grouped.completed.map((item) => (
                  <CompletedRow key={item.id} item={item} />
                ))}
              </div>
            </section>
          )}
        </>
      )}
    </div>
  )
}

function ActiveRow({ item, onPause }: { item: ActivityItem; onPause: () => void }) {
  const pct = item.progressPercent ?? 0
  return (
    <div className="flex items-center gap-4 p-3.5 px-4 rounded-[11px] border border-border bg-surface shadow-sm">
      <div className="relative w-[34px] flex-none aspect-[2/3] rounded-[2px] overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-[9px] text-text-3">
        <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-black/25" />
        EPB
      </div>

      <div className="w-[260px] flex-[1_1_200px] min-w-0">
        <div className="text-[13.5px] font-medium truncate text-text">{item.title}</div>
        <div className="text-xs text-text-3 truncate mt-[3px]">{item.author || 'Unknown author'}</div>
      </div>

      <div className="w-[170px] flex-[0_1_170px] min-w-0 font-mono text-[10.5px] text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-none px-2 py-[3px] rounded-[5px] bg-surface-3 font-mono text-[9.5px] text-text-2">
        {item.format}
      </div>

      <div className="flex-[1_1_160px] min-w-0">
        <div className="h-[3px] rounded-[2px] bg-border overflow-hidden">
          <div
            className="h-full rounded-[2px] bg-accent transition-all duration-300"
            style={{ width: `${Math.min(Math.max(pct, 0), 100)}%` }}
          />
        </div>
        <div className="font-mono text-[10px] text-text-3 mt-1.5 truncate">
          {item.progressText || `${pct}%`}
        </div>
      </div>

      <button
        type="button"
        onClick={onPause}
        className={cx(
          'flex-none inline-flex items-center h-7 px-3 rounded-[7px] border border-border text-xs text-text-2 hover:text-text cursor-pointer bg-surface',
          FOCUS_RING,
        )}
      >
        Pause
      </button>
    </div>
  )
}

function QueuedRow({ item, onCancel }: { item: ActivityItem; onCancel: () => void }) {
  return (
    <div className="flex items-center gap-4 py-2.5 px-4">
      <div className="relative w-[26px] flex-none aspect-[2/3] rounded-[2px] overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-[8px] text-text-3">
        EPB
      </div>

      <div className="flex-[0_1_260px] min-w-0 text-[13px] truncate text-text">{item.title}</div>

      <div className="flex-[0_1_170px] min-w-0 font-mono text-[10.5px] text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-none px-2 py-[3px] rounded-[5px] bg-surface-3 font-mono text-[9.5px] text-text-2">
        {item.format}
      </div>

      <div className="flex-1 min-w-0 text-xs text-text-3 truncate">
        {item.progressText || 'Waiting for source'}
      </div>

      <button
        type="button"
        onClick={onCancel}
        className={cx(
          'flex-none text-xs text-text-2 hover:text-error cursor-pointer',
          FOCUS_RING,
        )}
      >
        Cancel
      </button>
    </div>
  )
}

function FailedRow({ item, onRetry }: { item: ActivityItem; onRetry: () => void }) {
  return (
    <div className="flex items-center gap-4 p-3.5 px-4 rounded-[11px] border border-border bg-surface">
      <div className="relative w-[34px] flex-none aspect-[2/3] rounded-[2px] overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-[9px] text-text-3">
        EPB
      </div>

      <div className="flex-[0_1_260px] min-w-0 text-[13.5px] font-medium truncate text-text">
        {item.title}
      </div>

      <div className="flex-[0_1_170px] min-w-0 font-mono text-[10.5px] text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-1 min-w-0 flex items-center gap-2">
        <div className="w-[5px] h-[5px] rounded-full bg-error flex-none" />
        <div className="text-[12.5px] text-error truncate">
          {item.errorMessage || 'Operation failed'}
        </div>
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
          'inline-flex items-center h-7 px-3 rounded-[7px] border border-border text-xs text-text-2 hover:text-text cursor-pointer bg-surface',
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
      <div className="relative w-[26px] flex-none aspect-[2/3] rounded-[2px] overflow-hidden bg-surface-3 flex items-center justify-center font-mono text-[8px] text-text-3">
        EPB
      </div>

      <div className="flex-[0_1_260px] min-w-0 text-[13px] truncate text-text">{item.title}</div>

      <div className="flex-[0_1_170px] min-w-0 font-mono text-[10.5px] text-text-3 truncate">
        {item.sourceName}
      </div>

      <div className="flex-none px-2 py-[3px] rounded-[5px] bg-surface-3 font-mono text-[9.5px] text-text-2">
        {item.format}
      </div>

      <div className="flex-1 min-w-0 flex items-center gap-2">
        <div className="w-[5px] h-[5px] rounded-full bg-success flex-none" />
        <div className="text-[12.5px] text-text-2 truncate">Added to library</div>
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
