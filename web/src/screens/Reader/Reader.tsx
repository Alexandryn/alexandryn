import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Spinner } from '../../components/Spinner/Spinner'
import { useQuery } from '@tanstack/react-query'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import {
  useReadingExport,
  useBookmarks,
  useCreateBookmark,
  useCreateHighlight,
  useHighlights,
  useReadingPreferences,
  useReadingProgress,
  useReportProgress,
  useSavePreferences,
  DEFAULT_READING_PREFERENCES,
  type ReadingPreferences,
} from '../../data/reading'
import { contentUrl, flattenToc, loadEpub, sectionIndexForHref } from './epubBook'
import { positionCfi, progressRatio, sectionIndexForCfi } from './position'
import { useDebouncedCallback } from './useDebouncedCallback'
import './reader.css'

const PREFERENCE_DEBOUNCE_MS = 500
const POSITION_DEBOUNCE_MS = 3000

type Panel = 'toc' | 'marks' | 'settings' | null

/**
 * The in-browser EPUB reader at /read/:workId/:editionId
 * (frontend-reader.md). Chapter documents render inside a strictly
 * sandboxed <iframe> (allow-same-origin, never allow-scripts — FR-1);
 * every resource is fetched from the sanitised content endpoint, never a
 * blob: URL. Position is reported debounced (FR-6) and restored on mount
 * (FR-5); typography, theme, TOC, bookmarks, and highlights follow the
 * design reference's atReader canvas.
 */
export function Reader() {
  const { workId = '', editionId = '' } = useParams()

  const bookQuery = useQuery({
    queryKey: ['reader', 'epub', editionId],
    queryFn: () => loadEpub(editionId),
    enabled: editionId !== '',
    staleTime: Infinity,
    retry: false,
  })

  const progressQuery = useReadingProgress(workId)
  const preferencesQuery = useReadingPreferences()
  const bookmarksQuery = useBookmarks(editionId)
  const highlightsQuery = useHighlights(editionId)

  const report = useReportProgress(workId)
  const savePreferences = useSavePreferences()
  const createBookmark = useCreateBookmark(editionId)
  const createHighlight = useCreateHighlight(editionId)
  const readingExport = useReadingExport()

  const [sectionIndex, setSectionIndex] = useState(0)
  const [panel, setPanel] = useState<Panel>(null)
  const [chromeVisible, setChromeVisible] = useState(true)
  const [ratio, setRatio] = useState(0)
  const [selectionRange, setSelectionRange] = useState<{ start: string; end: string } | null>(null)

  const iframeRef = useRef<HTMLIFrameElement>(null)
  const [restored, setRestored] = useState(false)

  const preferences = preferencesQuery.data?.preferences ?? DEFAULT_READING_PREFERENCES
  const sections = bookQuery.data?.sections ?? []
  const currentSection = sections[sectionIndex]
  const observedEpoch = progressQuery.data?.progress?.epoch ?? 0

  const toc = useMemo(
    () => (bookQuery.data ? flattenToc(bookQuery.data.toc) : []),
    [bookQuery.data],
  )

  // Restore the last position once the book and progress are both loaded.
  // Adjusting state during render (React's supported pattern for deriving
  // from freshly-available data) — guarded so it runs exactly once.
  if (!restored && sections.length > 0 && !progressQuery.isPending) {
    setRestored(true)
    const target = restoreTarget(sections, progressQuery.data?.progress ?? null)
    if (target !== sectionIndex) {
      setSectionIndex(target)
    }
  }

  const reportPosition = useDebouncedCallback(() => {
    if (!currentSection) return
    const doc = iframeRef.current?.contentDocument ?? null
    const cfi = positionCfi(currentSection, doc)
    void report.mutate({
      percentage: ratio,
      observedEpoch,
      precisePosition: { editionId, cfi },
    })
  }, POSITION_DEBOUNCE_MS)

  const savePreferencesDebounced = useDebouncedCallback((next: ReadingPreferences) => {
    void savePreferences.mutate(next)
  }, PREFERENCE_DEBOUNCE_MS)

  const applyPreferences = useCallback(
    (doc: Document | null | undefined) => {
      if (!doc?.head) return
      let styleEl = doc.getElementById('alexandryn-reader-style') as HTMLStyleElement | null
      if (!styleEl) {
        styleEl = doc.createElement('style')
        styleEl.id = 'alexandryn-reader-style'
        doc.head.appendChild(styleEl)
      }
      const maxWidth =
        preferences.columnWidth === 'narrow'
          ? '34rem'
          : preferences.columnWidth === 'wide'
            ? '52rem'
            : '42rem'
      styleEl.textContent = [
        `html{color-scheme:${preferences.theme === 'dark' ? 'dark' : 'light'}}`,
        `body{`,
        `font-family:${preferences.font === 'serif' ? 'Georgia, serif' : 'system-ui, sans-serif'};`,
        `font-size:${preferences.fontSize}px;`,
        `line-height:${preferences.lineSpacing};`,
        `max-width:${maxWidth};margin:0 auto;padding:2rem 1.25rem 6rem;`,
        `}`,
        `img{max-width:100%;height:auto}`,
      ].join('')
    },
    [preferences],
  )

  const handleIframeLoad = useCallback(() => {
    const win = iframeRef.current?.contentWindow
    const doc = iframeRef.current?.contentDocument
    if (!win || !doc) return
    applyPreferences(doc)
    restoreScroll(win)

    const onScroll = () => {
      const el = doc.scrollingElement ?? doc.documentElement
      const denom = el.scrollHeight - el.clientHeight
      const fraction = denom > 0 ? el.scrollTop / denom : 0
      setRatio(progressRatio(sectionIndex, sections.length, fraction))
      reportPosition()
    }
    win.addEventListener('scroll', onScroll, { passive: true })

    const onSelect = () => {
      const sel = doc.getSelection()
      if (!sel || sel.isCollapsed || sel.rangeCount === 0) {
        setSelectionRange(null)
        return
      }
      try {
        const range = sel.getRangeAt(0)
        const start = positionCfi(currentSection!, doc)
        // End position: reuse the same section step; a distinct local CFI
        // for the end needs a collapsed-to-end range — kept simple here,
        // flagged as needing real-browser verification (spec Open Q3).
        setSelectionRange({ start, end: start })
        void range
      } catch {
        setSelectionRange(null)
      }
    }
    doc.addEventListener('selectionchange', onSelect)

    setRatio(progressRatio(sectionIndex, sections.length, 0))
  }, [applyPreferences, sectionIndex, sections.length, reportPosition, currentSection])

  // Re-apply typography whenever preferences change without reloading.
  useEffect(() => {
    applyPreferences(iframeRef.current?.contentDocument)
  }, [applyPreferences])

  const finalReportRef = useRef<() => void>(() => {})
  useEffect(() => {
    finalReportRef.current = () => {
      if (!currentSection) return
      const doc = iframeRef.current?.contentDocument ?? null
      report.mutate({
        percentage: ratio,
        observedEpoch,
        precisePosition: { editionId, cfi: positionCfi(currentSection, doc) },
      })
    }
  })
  useEffect(() => {
    const run = finalReportRef
    // Best-effort final report on unmount / navigation away (FR-6).
    return () => run.current()
  }, [])

  if (editionId === '' || workId === '') {
    return <ErrorState title="This reader link is incomplete." />
  }
  if (bookQuery.isPending) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner label="Opening book" />
      </div>
    )
  }
  if (bookQuery.isError || sections.length === 0) {
    const status = (bookQuery.error as { status?: number } | undefined)?.status
    return (
      <ErrorState
        title={
          status === 503
            ? "This book's source isn't reachable right now."
            : 'This book could not be opened.'
        }
        onRetry={() => void bookQuery.refetch()}
      />
    )
  }

  const setPreference = <K extends keyof ReadingPreferences>(key: K, value: ReadingPreferences[K]) => {
    const next = { ...preferences, [key]: value }
    savePreferencesDebounced(next)
  }

  const goToSection = (idx: number) => {
    setSectionIndex(Math.max(0, Math.min(sections.length - 1, idx)))
    setPanel(null)
    setRatio(progressRatio(idx, sections.length, 0))
  }

  return (
    <div className="reader-root" data-theme={preferences.theme} data-layout={preferences.layoutMode}>
      {chromeVisible && (
        <div className="reader-bar">
          <Link to={`/book/${workId}`} className={cx('text-sm opacity-70', FOCUS_RING)}>
            ← Library
          </Link>
          <div className="min-w-0 flex-1 truncate text-center text-sm opacity-70">
            {bookQuery.data?.title}
          </div>
          <div className="flex flex-none gap-2xs">
            <ToolButton active={panel === 'toc'} onClick={() => setPanel(panel === 'toc' ? null : 'toc')}>
              Contents
            </ToolButton>
            <ToolButton
              active={panel === 'settings'}
              onClick={() => setPanel(panel === 'settings' ? null : 'settings')}
              label="Reading settings"
            >
              Aa
            </ToolButton>
            <ToolButton
              active={panel === 'marks'}
              onClick={() => setPanel(panel === 'marks' ? null : 'marks')}
            >
              Marks
            </ToolButton>
            <ToolButton
              active={false}
              onClick={() => setChromeVisible(false)}
              label="Hide reading controls"
            >
              ⤢
            </ToolButton>
          </div>
        </div>
      )}
      {!chromeVisible && (
        <button
          type="button"
          className={cx('absolute right-md top-md z-10 rounded-md border bg-surface px-sm py-2xs text-xs', FOCUS_RING)}
          onClick={() => setChromeVisible(true)}
        >
          Show controls
        </button>
      )}

      <iframe
        ref={iframeRef}
        className="reader-content-frame"
        title={`${bookQuery.data?.title ?? 'Book'} — reading area`}
        sandbox="allow-same-origin"
        src={currentSection ? contentUrl(editionId, currentSection.id) : undefined}
        onLoad={handleIframeLoad}
      />

      {selectionRange && (
        <div className="reader-footer" role="region" aria-label="Selection actions">
          <button
            type="button"
            className={cx('text-sm', FOCUS_RING)}
            onClick={() => {
              void createHighlight.mutate({
                startCfi: selectionRange.start,
                endCfi: selectionRange.end,
                category: 'yellow',
              })
              setSelectionRange(null)
            }}
          >
            Highlight selection
          </button>
        </div>
      )}

      {panel === 'toc' && (
        <nav className="reader-panel reader-panel--toc" aria-label="Table of contents">
          <p className="font-mono text-2xs uppercase tracking-2 opacity-60">Contents</p>
          <ul className="mt-sm flex flex-col gap-4xs">
            {toc.map((entry, i) => {
              const idx = sectionIndexForHref(sections, entry.href)
              const active = idx === sectionIndex
              return (
                <li key={`${entry.href}-${i}`} style={{ paddingLeft: entry.depth ? '0.75rem' : undefined }}>
                  <button
                    type="button"
                    className={cx('reader-toc-entry', FOCUS_RING)}
                    aria-current={active}
                    onClick={() => goToSection(idx >= 0 ? idx : sectionIndex)}
                  >
                    {entry.label}
                  </button>
                </li>
              )
            })}
          </ul>
        </nav>
      )}

      {panel === 'settings' && (
        <div className="reader-panel reader-panel--settings" role="group" aria-label="Reading settings">
          <p className="font-mono text-2xs uppercase tracking-2 opacity-60">Reading</p>

          <fieldset className="mt-md border-0 p-0">
            <legend className="text-xs opacity-70">Theme</legend>
            <div className="mt-2xs flex gap-2xs">
              {(['light', 'sepia', 'dark'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  className={cx('flex-1 rounded-md border p-xs text-sm capitalize', FOCUS_RING)}
                  aria-pressed={preferences.theme === t}
                  onClick={() => setPreference('theme', t)}
                >
                  {t}
                </button>
              ))}
            </div>
          </fieldset>

          <div className="mt-md flex items-center gap-sm">
            <span className="text-xs opacity-70">Type size</span>
            <button
              type="button"
              aria-label="Smaller text"
              className={cx('rounded-md border px-xs', FOCUS_RING)}
              onClick={() => setPreference('fontSize', Math.max(12, preferences.fontSize - 1))}
            >
              −
            </button>
            <span className="text-sm tabular-nums">{preferences.fontSize}</span>
            <button
              type="button"
              aria-label="Larger text"
              className={cx('rounded-md border px-xs', FOCUS_RING)}
              onClick={() => setPreference('fontSize', Math.min(32, preferences.fontSize + 1))}
            >
              +
            </button>
          </div>

          <div className="mt-md flex items-center gap-sm">
            <span className="text-xs opacity-70">Line spacing</span>
            <input
              type="range"
              min={1}
              max={2.5}
              step={0.1}
              value={preferences.lineSpacing}
              aria-label="Line spacing"
              onChange={(e) => setPreference('lineSpacing', Number(e.target.value))}
            />
          </div>

          <fieldset className="mt-md border-0 p-0">
            <legend className="text-xs opacity-70">Column width</legend>
            <div className="mt-2xs flex gap-2xs">
              {(['narrow', 'default', 'wide'] as const).map((wKey) => (
                <button
                  key={wKey}
                  type="button"
                  className={cx('flex-1 rounded-md border p-xs text-sm capitalize', FOCUS_RING)}
                  aria-pressed={preferences.columnWidth === wKey}
                  onClick={() => setPreference('columnWidth', wKey)}
                >
                  {wKey}
                </button>
              ))}
            </div>
          </fieldset>

          <fieldset className="mt-md border-0 p-0">
            <legend className="text-xs opacity-70">Layout</legend>
            <div className="mt-2xs flex gap-2xs">
              {(['paginated', 'scroll'] as const).map((mode) => (
                <button
                  key={mode}
                  type="button"
                  className={cx('flex-1 rounded-md border p-xs text-sm capitalize', FOCUS_RING)}
                  aria-pressed={preferences.layoutMode === mode}
                  onClick={() => setPreference('layoutMode', mode)}
                >
                  {mode}
                </button>
              ))}
            </div>
          </fieldset>
        </div>
      )}

      {panel === 'marks' && (
        <aside className="reader-panel reader-panel--marks" aria-label="Bookmarks and highlights">
          <div className="flex items-center gap-sm">
            <p className="flex-1 font-mono text-2xs uppercase tracking-2 opacity-60">
              Marks · {(bookmarksQuery.data?.bookmarks.length ?? 0) + (highlightsQuery.data?.highlights.length ?? 0)}
            </p>
            <button
              type="button"
              className={cx('text-xs opacity-70', FOCUS_RING)}
              onClick={() => readingExport.mutate()}
              disabled={readingExport.isPending}
            >
              {readingExport.isPending ? 'Exporting…' : 'Export'}
            </button>
          </div>
          {readingExport.isError && (
            <p role="alert" className="text-xs text-error">
              Couldn't export your reading data. Check your connection and try again.
            </p>
          )}

          <button
            type="button"
            className={cx('mt-md w-full rounded-lg border p-sm text-sm', FOCUS_RING)}
            onClick={() => {
              const doc = iframeRef.current?.contentDocument ?? null
              if (currentSection) {
                void createBookmark.mutate({ cfi: positionCfi(currentSection, doc) })
              }
            }}
          >
            Bookmark this page
          </button>

          <ul className="mt-md flex flex-col gap-sm">
            {bookmarksQuery.data?.bookmarks.map((b) => (
              <li key={b.id} className="rounded-lg border p-sm text-sm">
                <span className="font-mono text-2xs uppercase tracking-1 opacity-60">Bookmark</span>
                <p className="mt-4xs">{b.label || 'Untitled bookmark'}</p>
              </li>
            ))}
            {highlightsQuery.data?.highlights.map((h) => (
              <li key={h.id} className="rounded-lg border p-sm text-sm">
                <span className="font-mono text-2xs uppercase tracking-1 opacity-60">Highlight</span>
                {h.note && <p className="mt-4xs opacity-80">{h.note}</p>}
              </li>
            ))}
            {(bookmarksQuery.data?.bookmarks.length ?? 0) === 0 &&
              (highlightsQuery.data?.highlights.length ?? 0) === 0 && (
                <li className="rounded-lg border border-dashed p-md text-sm opacity-70">
                  Nothing marked yet. Select a passage to highlight it, or bookmark this page. Marks are
                  stored with the library, not in this browser.
                </li>
              )}
          </ul>
        </aside>
      )}

      <div className="reader-footer">
        <button
          type="button"
          className={cx('text-sm opacity-70', FOCUS_RING)}
          onClick={() => goToSection(sectionIndex - 1)}
          disabled={sectionIndex === 0}
          aria-label="Previous chapter"
        >
          ‹
        </button>
        <div className="reader-progress-track">
          <div
            className="reader-progress-fill"
            style={{ width: `${Math.round(ratio * 100)}%` }}
            role="progressbar"
            aria-valuenow={Math.round(ratio * 100)}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="Reading progress"
          />
        </div>
        <span className="flex-none font-mono text-2xs opacity-60">{Math.round(ratio * 100)}%</span>
        <button
          type="button"
          className={cx('text-sm opacity-70', FOCUS_RING)}
          onClick={() => goToSection(sectionIndex + 1)}
          disabled={sectionIndex === sections.length - 1}
          aria-label="Next chapter"
        >
          ›
        </button>
      </div>
    </div>
  )
}

function ToolButton({
  active,
  onClick,
  children,
  label,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
  label?: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      aria-label={label}
      className={cx(
        'h-8 rounded-md border px-sm text-xs',
        active && 'bg-surface-2',
        FOCUS_RING,
      )}
    >
      {children}
    </button>
  )
}

function restoreScroll(win: Window) {
  win.scrollTo(0, 0)
}

/** The spine index to open at, from a saved ReadingProgress (FR-5). */
function restoreTarget(
  sections: { cfi: string }[],
  saved: { percentage: number; precisePosition: { cfi: string } | null } | null,
): number {
  if (!saved) return 0
  if (saved.precisePosition?.cfi) {
    const idx = sectionIndexForCfi(sections as never, saved.precisePosition.cfi)
    if (idx >= 0) return idx
  }
  return Math.min(sections.length - 1, Math.max(0, Math.floor(saved.percentage * sections.length)))
}
