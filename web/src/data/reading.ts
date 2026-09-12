// The reader's data layer. Every server value goes through a TanStack Query
// hook; the reader's own `X-Device-Id` header is generated once and kept in
// localStorage.

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { deleteRequest, getBlob, getJson, postJson, putJson } from './http'

const DEVICE_ID_KEY = 'alexandryn.reader.deviceId'

/**
 * A stable per-device UUID v4, created on first use and persisted in
 * localStorage. Never displayed, never treated as identity — it
 * only satisfies the reading progress API required header.
 */
export function readerDeviceId(): string {
  try {
    const existing = localStorage.getItem(DEVICE_ID_KEY)
    if (existing && /^[0-9a-f-]{36}$/i.test(existing)) {
      return existing
    }
    const fresh = crypto.randomUUID()
    localStorage.setItem(DEVICE_ID_KEY, fresh)
    return fresh
  } catch {
    // Private mode / storage disabled — a session-only id still lets the
    // reader function; it just won't persist preferences across reloads.
    return crypto.randomUUID()
  }
}

function deviceHeaders(): Record<string, string> {
  return { 'X-Device-Id': readerDeviceId() }
}

// --- types (mirror the backend wire shapes) --------------------------

export interface PrecisePosition {
  editionId: string
  cfi: string
}

export interface ReadingProgress {
  percentage: number
  epoch: number
  precisePosition: PrecisePosition | null
  observedAt: string
}

export type ReconcileOutcome = 'advanced' | 'unchanged' | 'rejected' | 'overridden'

export interface ProgressReportResult {
  progress: ReadingProgress
  outcome: ReconcileOutcome
}

export interface ProgressReportInput {
  percentage: number
  observedEpoch: number
  precisePosition?: PrecisePosition | null
  override?: boolean
}

export interface Bookmark {
  id: string
  editionId: string
  cfi: string
  label: string
  createdAt: string
}

export interface Highlight {
  id: string
  editionId: string
  startCfi: string
  endCfi: string
  note: string
  category: string
  createdAt: string
}

export type ReaderTheme = 'light' | 'sepia' | 'dark'
export type LayoutMode = 'paginated' | 'scroll'
export type ColumnWidth = 'narrow' | 'default' | 'wide'

export interface ReadingPreferences {
  font: string
  fontSize: number
  lineSpacing: number
  theme: ReaderTheme
  layoutMode: LayoutMode
  columnWidth: ColumnWidth
}

/** System defaults, matching the backend's `defaultPreferences` and the
 * design reference's atReader canvas. */
export const DEFAULT_READING_PREFERENCES: ReadingPreferences = {
  font: 'serif',
  fontSize: 19,
  lineSpacing: 1.5,
  theme: 'light',
  layoutMode: 'paginated',
  columnWidth: 'default',
}

// --- query keys -----------------------------------------------------

export const readingKeys = {
  progress: (workId: string) => ['reading', 'progress', workId] as const,
  bookmarks: (editionId: string) => ['reading', 'bookmarks', editionId] as const,
  highlights: (editionId: string) => ['reading', 'highlights', editionId] as const,
  preferences: ['reading', 'preferences'] as const,
}

// --- progress -------------------------------------------------------

export function useReadingProgress(workId: string) {
  return useQuery({
    queryKey: readingKeys.progress(workId),
    queryFn: () =>
      getJson<{ progress: ReadingProgress | null }>(
        `/api/v1/reading/works/${encodeURIComponent(workId)}/progress`,
      ),
    staleTime: 30_000,
  })
}

export function useReportProgress(workId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: ProgressReportInput) =>
      postJson<ProgressReportResult>(
        `/api/v1/reading/works/${encodeURIComponent(workId)}/progress`,
        input,
        deviceHeaders(),
      ),
    onSuccess: (result) => {
      qc.setQueryData(readingKeys.progress(workId), { progress: result.progress })
    },
  })
}

// --- bookmarks -----------------------------------------------------

export function useBookmarks(editionId: string) {
  return useQuery({
    queryKey: readingKeys.bookmarks(editionId),
    queryFn: () =>
      getJson<{ bookmarks: Bookmark[] }>(
        `/api/v1/reading/editions/${encodeURIComponent(editionId)}/bookmarks`,
      ),
  })
}

export function useCreateBookmark(editionId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: { cfi: string; label?: string | null }) =>
      postJson<{ bookmark: Bookmark }>(
        `/api/v1/reading/editions/${encodeURIComponent(editionId)}/bookmarks`,
        input,
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: readingKeys.bookmarks(editionId) }),
  })
}

export function useDeleteBookmark(editionId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (bookmarkId: string) =>
      deleteRequest(`/api/v1/reading/bookmarks/${encodeURIComponent(bookmarkId)}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: readingKeys.bookmarks(editionId) }),
  })
}

// --- highlights --------------------------------------------------

export function useHighlights(editionId: string) {
  return useQuery({
    queryKey: readingKeys.highlights(editionId),
    queryFn: () =>
      getJson<{ highlights: Highlight[] }>(
        `/api/v1/reading/editions/${encodeURIComponent(editionId)}/highlights`,
      ),
  })
}

export function useCreateHighlight(editionId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (input: {
      startCfi: string
      endCfi: string
      note?: string | null
      category?: string | null
    }) =>
      postJson<{ highlight: Highlight }>(
        `/api/v1/reading/editions/${encodeURIComponent(editionId)}/highlights`,
        input,
      ),
    onSuccess: () => qc.invalidateQueries({ queryKey: readingKeys.highlights(editionId) }),
  })
}

export function useDeleteHighlight(editionId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (highlightId: string) =>
      deleteRequest(`/api/v1/reading/highlights/${encodeURIComponent(highlightId)}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: readingKeys.highlights(editionId) }),
  })
}

// --- preferences ------------------------------------------------

export function useReadingPreferences() {
  return useQuery({
    queryKey: readingKeys.preferences,
    queryFn: () =>
      getJson<{ preferences: ReadingPreferences }>(
        '/api/v1/reading/preferences',
        deviceHeaders(),
      ),
  })
}

export function useSavePreferences() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (prefs: ReadingPreferences) =>
      putJson<{ preferences: ReadingPreferences }>(
        '/api/v1/reading/preferences',
        prefs,
        deviceHeaders(),
      ),
    onSuccess: (result) => qc.setQueryData(readingKeys.preferences, result),
  })
}

// --- export ---------------------------------------------------

/**
 * Fetches the reading-data export document — through the authenticated
 * HTTP layer, so the bearer token and X-Library-Id are sent — and hands it
 * to the browser as a file download. Web/LAN client only.
 */
export async function downloadReadingExport(): Promise<void> {
  const { blob, filename } = await getBlob('/api/v1/reading/export')
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename ?? 'alexandryn-reading-export.json'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}

/** Mutation wrapper so the Reader can show pending/error state. */
export function useReadingExport() {
  return useMutation({ mutationFn: downloadReadingExport })
}
