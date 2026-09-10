import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getJson, postJson } from './http'

export interface SystemEventPayload {
  kind?: string
  title?: string
  book_title?: string
  name?: string
  author?: string
  creator?: string
  source_name?: string
  source?: string
  source_id?: string
  format?: string
  file_format?: string
  progress_percent?: number
  // Note: progress percentage fields (percentage, pct) are stripped server-side in
  // SanitizedPayload (Constitution §8, ADR 0031) to protect reading privacy and prevent
  // leaking granular positions. Kept here defensively for event payload typing.
  percentage?: number
  pct?: number
  progress_text?: string
  st?: string
  error?: string
  last_error?: string
  reason?: string
  work_id?: string
  duration_ms?: number
  items_total?: number
  [key: string]: unknown
}

export interface SystemEvent {
  id: number
  event_kind: string
  job_id?: string | null
  library_id?: string | null
  user_id?: string | null
  payload: SystemEventPayload
  created_at: string
  purge_at: string
}

export interface ActivityEventsResponse {
  events: SystemEvent[]
}

export interface ActivityItem {
  id: string
  jobId?: string
  workId?: string
  title: string
  author?: string
  sourceName: string
  format: string
  status: 'active' | 'queued' | 'failed' | 'completed'
  progressPercent?: number
  progressText?: string
  errorMessage?: string
  completedAt?: string
  coverUrl?: string
}

export interface ActivityGrouped {
  active: ActivityItem[]
  queued: ActivityItem[]
  failed: ActivityItem[]
  completed: ActivityItem[]
}

export function parseActivityEvents(events: SystemEvent[]): ActivityGrouped {
  const latestByJob = new Map<string, SystemEvent>()
  const nonJobEvents: SystemEvent[] = []

  for (const ev of events) {
    if (ev.job_id) {
      if (!latestByJob.has(ev.job_id)) {
        latestByJob.set(ev.job_id, ev)
      }
    } else {
      nonJobEvents.push(ev)
    }
  }

  const items: ActivityItem[] = []

  for (const [jobId, ev] of latestByJob.entries()) {
    items.push(eventToItem(ev, jobId))
  }

  for (const ev of nonJobEvents) {
    items.push(eventToItem(ev, `event-${ev.id}`))
  }

  const grouped: ActivityGrouped = {
    active: [],
    queued: [],
    failed: [],
    completed: [],
  }

  for (const item of items) {
    grouped[item.status].push(item)
  }

  return grouped
}

function eventToItem(ev: SystemEvent, id: string): ActivityItem {
  const p = ev.payload || {}
  const title = p.title || p.book_title || p.name || p.kind || ev.event_kind
  const author = p.author || p.creator
  const sourceName = p.source_name || p.source || (p.source_id ? `Source ${p.source_id}` : 'Local')
  const format = (p.format || p.file_format || 'EPUB').toUpperCase()

  let status: ActivityItem['status'] = 'queued'
  if (ev.event_kind === 'job.running' || ev.event_kind === 'import.started') {
    status = 'active'
  } else if (ev.event_kind === 'job.failed' || ev.event_kind === 'job.dead_letter') {
    status = 'failed'
  } else if (ev.event_kind === 'job.completed' || ev.event_kind === 'import.finished') {
    status = 'completed'
  } else {
    status = 'queued'
  }

  const pct = p.progress_percent ?? p.percentage ?? p.pct
  const progressText = p.progress_text || p.st || (pct !== undefined ? `${pct}%` : undefined)
  const errorMessage = p.error || p.last_error || p.reason

  return {
    id,
    jobId: ev.job_id || undefined,
    workId: p.work_id,
    title,
    author,
    sourceName,
    format,
    status,
    progressPercent: pct,
    progressText,
    errorMessage,
    completedAt: ev.created_at,
  }
}

export const ACTIVITY_QUERY_KEY = ['activity', 'events'] as const

// Polling cadence (audit 0016 #102): the badge lives in the always-mounted
// sidebar, so a fixed 10s interval meant every desktop client hit
// /activity/events every 10s for the whole session, idle or backgrounded.
// Now: no polling while the tab is hidden, 5s while a job is active, and a
// slow 60s floor otherwise so a job started elsewhere still surfaces
// within a minute. The Activity screen passes `active: true` for the
// faster cadence while it is open.
const ACTIVE_POLL_MS = 5000
const IDLE_POLL_MS = 60000

/**
 * The adaptive polling cadence (audit 0016 #102): stop while the tab is
 * hidden, poll fast while a job is running or the Activity screen is
 * open, and fall back to a slow floor otherwise.
 */
export function activityPollInterval(
  events: SystemEvent[] | undefined,
  opts: { active?: boolean },
  hidden: boolean,
): number | false {
  if (hidden) return false
  if (parseActivityEvents(events ?? []).active.length > 0) return ACTIVE_POLL_MS
  return opts.active ? ACTIVE_POLL_MS : IDLE_POLL_MS
}

export function useActivityEvents(opts: { active?: boolean } = {}) {
  return useQuery({
    queryKey: ACTIVITY_QUERY_KEY,
    queryFn: async () => {
      const data = await getJson<ActivityEventsResponse>('/api/v1/activity/events?limit=50')
      return data.events || []
    },
    refetchInterval: (query) =>
      activityPollInterval(
        query.state.data,
        opts,
        typeof document !== 'undefined' && document.visibilityState === 'hidden',
      ),
    refetchOnWindowFocus: true,
  })
}

export function useActivityBadge() {
  const { data: events = [] } = useActivityEvents()
  // Memoize grouped events to prevent downstream memo breakage across poll ticks (audit 0016 #214).
  const hasActiveOrFailed = useMemo(() => {
    const grouped = parseActivityEvents(events)
    return grouped.active.length > 0 || grouped.failed.length > 0
  }, [events])
  return { hasActiveOrFailed }
}

export function usePauseAll() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => postJson<{ paused: boolean }>('/api/v1/activity/pause-all'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ACTIVITY_QUERY_KEY })
    },
  })
}

export function useCancelJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (jobId: string) =>
      postJson<{ cancelled: boolean }>(`/api/v1/activity/jobs/${jobId}/cancel`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ACTIVITY_QUERY_KEY })
    },
  })
}

export function useRetryJob() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (jobId: string) =>
      postJson<{ new_job_id: string }>(`/api/v1/activity/jobs/${jobId}/retry`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ACTIVITY_QUERY_KEY })
    },
  })
}

export function useClearCompleted() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => postJson<{ cleared_count: number }>('/api/v1/activity/jobs/clear-completed'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ACTIVITY_QUERY_KEY })
    },
  })
}
