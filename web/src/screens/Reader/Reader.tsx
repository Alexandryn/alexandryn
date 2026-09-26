import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { ErrorState } from '../../components/ErrorState/ErrorState'
import { Spinner } from '../../components/Spinner/Spinner'
import {
  BookmarkIcon,
  BookOpenIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  MaximizeIcon,
} from '../../components/Icon'
import { useQuery } from '@tanstack/react-query'
import { cx } from '../../lib/cx'
import { FOCUS_RING } from '../../lib/focusRing'
import {
  useReadingExport,
  useBookmarks,
  useCreateBookmark,
  useCreateHighlight,
  useHighlights,
  useReaderContentSession,
  useReadingPreferences,
  useReadingProgress,
  useReportProgress,
  useSavePreferences,
  CONTENT_GRANT_SAFE_MS,
  DEFAULT_READING_PREFERENCES,
  type ReadingPreferences,
} from '../../data/reading'
import type { EpubSection } from '../../vendor/foliate/epub'
import {
  contentUrl,
  flattenToc,
  loadEpub,
  sectionIndexForContentPath,
  sectionIndexForHref,
} from './epubBook'
import { applyReaderCss } from './readerCss'
import {
  positionCfi,
  progressRatio,
  restoreScroll,
  sectionIndexForCfi,
  sectionScrollFraction,
  selectionCfis,
} from './position'
import { useDebouncedCallback } from './useDebouncedCallback'
import './reader.css'

const PREFERENCE_DEBOUNCE_MS = 500
const NO_SECTIONS: EpubSection[] = []
const POSITION_DEBOUNCE_MS = 3000

type Panel = 'toc' | 'marks' | 'settings' | null

/**
 * The in-browser EPUB reader at /read/:workId/:editionId.
 * Chapter documents render inside a strictly sandboxed <iframe>
 * (allow-same-origin, never allow-scripts); every resource is fetched
 * from the sanitised content endpoint, never a blob: URL. Position is
 * reported debounced and restored on mount; typography, theme, TOC,
 * bookmarks, and highlights follow the design reference's atReader canvas.
 */
export function Reader() {
  const { workId = '', editionId = '' } = useParams()

  const bookQuery = useQuery({
    queryKey: ['reader', 'epub', editionId],
    queryFn: ({ signal }) => loadEpub(editionId, signal),
    enabled: editionId !== '',
    staleTime: Infinity,
    retry: false,
  })

  // The iframe authenticates by a grant cookie this issues; its src is
  // withheld until the first grant lands.
  const contentSession = useReaderContentSession(editionId)
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
  // The saved intra-chapter scroll offset is applied once, after the
  // restored chapter's document has loaded; later
  // chapter navigation still opens at the top. loadedSection tracks which
  // spine index the iframe currently holds a loaded document for, so the
  // restore fires whether or not restoreTarget changed sectionIndex.
  const scrollRestoreDone = useRef(false)
  const [loadedSection, setLoadedSection] = useState<number | null>(null)
  // What the iframe holds: `src` is the attribute last set on it, `want`
  // the chapter URL it satisfies, `index` that chapter's spine index. It
  // trails sectionIndex while a stale grant is re-issued, and a link in
  // the book can move the frame on its own, so anything measured inside
  // the frame (position, selection) keys off this, never off the chapter
  // the chrome has moved to.
  const [frame, setFrame] = useState<{ src: string; want: string; index: number } | null>(null)
  const frameSrc = frame?.src
  const [grantFailed, setGrantFailed] = useState(false)

  const preferences = preferencesQuery.data?.preferences ?? DEFAULT_READING_PREFERENCES
  const sections = bookQuery.data?.sections ?? NO_SECTIONS
  const currentSection = sections[sectionIndex]
  const loadedSec = loadedSection !== null ? sections[loadedSection] : undefined
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
    if (!loadedSec) return
    const doc = iframeRef.current?.contentDocument ?? null
    const cfi = positionCfi(loadedSec, doc)
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
      if (!doc) return
      const maxWidth =
        preferences.columnWidth === 'narrow'
          ? '34rem'
          : preferences.columnWidth === 'wide'
            ? '52rem'
            : '42rem'
      const css = [
        `html{color-scheme:${preferences.theme === 'dark' ? 'dark' : 'light'}}`,
        `body{`,
        `font-family:${preferences.font === 'serif' ? 'Georgia, serif' : 'system-ui, sans-serif'};`,
        `font-size:${preferences.fontSize}px;`,
        `line-height:${preferences.lineSpacing};`,
        `max-width:${maxWidth};margin:0 auto;padding:2rem 1.25rem 6rem;`,
        `}`,
        `img{max-width:100%;height:auto}`,
        // A cover the sanitiser converted from an SVG wrapper: fit it to
        // the page (inside the body's vertical padding), as its viewBox did.
        `img.alx-svg-cover{display:block;margin:0 auto;width:auto;max-height:calc(100vh - 8rem)}`,
      ].join('')
      applyReaderCss(doc, css)
    },
    [preferences],
  )

  const cleanupIframeListenersRef = useRef<(() => void) | null>(null)
  // These refs mirror the latest render's values so the iframe `load`
  // handler and the scroll/selection listeners it installs (all off a
  // stable useCallback) read current data without re-subscribing. Synced
  // in a layout effect: after DOM mutation, before paint, and well before
  // the async iframe `load` event or any user interaction with the frame.
  const sectionsLengthRef = useRef(sections.length)
  const sectionsRef = useRef(sections)
  const frameIndexRef = useRef<number | null>(null)
  useLayoutEffect(() => {
    sectionsLengthRef.current = sections.length
    sectionsRef.current = sections
    frameIndexRef.current = frame?.index ?? null
  }, [sections, frame])

  const handleIframeLoad = useCallback(() => {
    cleanupIframeListenersRef.current?.()
    const win = iframeRef.current?.contentWindow
    const doc = iframeRef.current?.contentDocument
    if (!win || !doc) return
    applyPreferences(doc)

    // The section this document is — fixed for the listeners below, so a
    // later chapter change cannot re-key measurements of this document. A
    // link in the book may have navigated the frame itself: read back
    // where it is, follow it with the chrome if it is another spine
    // section, and measure nothing in a document that is not one.
    let frameIndex = frameIndexRef.current
    let path: string | null = null
    try {
      path = win.location.pathname
    } catch {
      // Not readable (not same-origin): leave the frame's own record.
    }
    if (path?.startsWith('/api/')) {
      const sections = sectionsRef.current
      const idx = sectionIndexForContentPath(sections, editionId, path)
      if (idx !== frameIndex) {
        frameIndex = idx === -1 ? null : idx
        if (idx !== -1) {
          const want = contentUrl(editionId, sections[idx]!.id)
          setFrame((f) => (f ? { ...f, want, index: idx } : f))
          setSectionIndex(idx)
        }
      }
    }
    if (frameIndex === null) {
      setLoadedSection(null)
      return
    }
    const frameSection = sectionsRef.current[frameIndex]

    // Every load opens at the top; the one-shot intra-chapter restore is
    // applied by the effect below once this section's document is in.
    restoreScroll(win, 0)
    setLoadedSection(frameIndex)

    const onScroll = () => {
      const el = doc.scrollingElement ?? doc.documentElement
      if (!el) return
      const denom = el.scrollHeight - el.clientHeight
      const fraction = denom > 0 ? el.scrollTop / denom : 0
      setRatio(progressRatio(frameIndex, sectionsLengthRef.current, fraction))
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
        if (frameSection) {
          setSelectionRange(selectionCfis(frameSection, doc, sel.getRangeAt(0)))
        }
      } catch {
        setSelectionRange(null)
      }
    }
    doc.addEventListener('selectionchange', onSelect)

    cleanupIframeListenersRef.current = () => {
      try {
        win.removeEventListener('scroll', onScroll)
        doc.removeEventListener('selectionchange', onSelect)
      } catch {
        // Window or document may already be torn down
      }
    }

    setRatio(progressRatio(frameIndex, sectionsLengthRef.current, 0))
  }, [applyPreferences, reportPosition, editionId])

  useEffect(() => {
    return () => {
      cleanupIframeListenersRef.current?.()
    }
  }, [])

  // One-shot intra-chapter scroll restore. Runs after
  // the restored section's document is loaded — covers both the case
  // where restoreTarget moved sectionIndex (a fresh load fires) and the
  // case where the target was the section already on screen (no reload,
  // so handleIframeLoad never runs again).
  useEffect(() => {
    if (!restored || scrollRestoreDone.current) return
    if (progressQuery.isPending || loadedSection !== sectionIndex) return
    scrollRestoreDone.current = true
    const win = iframeRef.current?.contentWindow
    const saved = progressQuery.data?.progress
    if (win && saved) {
      restoreScroll(win, sectionScrollFraction(saved.percentage, sectionIndex, sections.length))
    }
  }, [
    restored,
    loadedSection,
    sectionIndex,
    sections.length,
    progressQuery.isPending,
    progressQuery.data,
  ])

  // The chapter URL the iframe should show, and the one it does show. A
  // grant older than CONTENT_GRANT_SAFE_MS may already have expired (timers
  // lag the cookie's wall-clock lifetime across a system suspend), so a
  // chapter change then waits for a fresh grant rather than loading a 401.
  const wantedSrc =
    currentSection && contentSession.data !== undefined
      ? contentUrl(editionId, currentSection.id)
      : undefined
  const { dataUpdatedAt: grantIssuedAt, refetch: reissueGrant } = contentSession
  const frameWant = frame?.want
  useEffect(() => {
    if (wantedSrc === undefined || wantedSrc === frameWant) return
    let cancelled = false
    const grant =
      Date.now() - grantIssuedAt < CONTENT_GRANT_SAFE_MS
        ? Promise.resolve({ isError: false })
        : reissueGrant()
    void grant.then((result) => {
      if (cancelled) return
      // A failed re-issue leaves the previous chapter framed and says so,
      // rather than silently showing chapter N under chapter N+1's title.
      setGrantFailed(result.isError)
      if (result.isError) return
      // The frame may still carry this very src attribute after a book
      // link moved it elsewhere; React will not re-set an unchanged
      // attribute, so navigate it directly.
      if (wantedSrc === frameSrc && iframeRef.current) iframeRef.current.src = wantedSrc
      setFrame({ src: wantedSrc, want: wantedSrc, index: sectionIndex })
    })
    return () => {
      cancelled = true
    }
  }, [wantedSrc, frameWant, frameSrc, sectionIndex, grantIssuedAt, reissueGrant])

  // On success grantIssuedAt moves, and the effect above frames the chapter.
  const retryGrant = () => {
    setGrantFailed(false)
    void reissueGrant().then((result) => {
      if (result.isError) setGrantFailed(true)
    })
  }

  // Re-apply typography whenever preferences change without reloading.
  useEffect(() => {
    applyPreferences(iframeRef.current?.contentDocument)
  }, [applyPreferences])

  const finalReportRef = useRef<() => void>(() => {})
  useEffect(() => {
    finalReportRef.current = () => {
      if (!loadedSec) return
      const doc = iframeRef.current?.contentDocument ?? null
      report.mutate({
        percentage: ratio,
        observedEpoch,
        precisePosition: { editionId, cfi: positionCfi(loadedSec, doc) },
      })
    }
  })
  useEffect(() => {
    const run = finalReportRef
    // Best-effort final report on unmount / navigation away.
    return () => run.current()
  }, [])

  if (editionId === '' || workId === '') {
    return <ErrorState title="This reader link is incomplete." />
  }
  if (bookQuery.isPending || contentSession.isPending) {
    return (
      <div className="flex h-full items-center justify-center" role="status">
        <h1 className="sr-only">Opening book</h1>
        <Spinner label="Opening book" />
      </div>
    )
  }
  const sessionFailed = contentSession.isError && contentSession.data === undefined
  if (bookQuery.isError || sections.length === 0 || sessionFailed) {
    // Only the book's own 503 means its source is unreachable; a 503 from
    // the grant endpoint means the reader itself is still starting.
    const bookStatus = (bookQuery.error as { status?: number } | null)?.status
    return (
      <ErrorState
        title={
          bookStatus === 503
            ? "This book's source isn't reachable right now."
            : 'This book could not be opened.'
        }
        onRetry={() => {
          if (bookQuery.isError) void bookQuery.refetch()
          if (sessionFailed) void contentSession.refetch()
        }}
      />
    )
  }

  const setPreference = <K extends keyof ReadingPreferences>(
    key: K,
    value: ReadingPreferences[K],
  ) => {
    const next = { ...preferences, [key]: value }
    savePreferencesDebounced(next)
  }

  const goToSection = (idx: number) => {
    setSectionIndex(Math.max(0, Math.min(sections.length - 1, idx)))
    setPanel(null)
    setRatio(progressRatio(idx, sections.length, 0))
  }

  return (
    <div
      className="reader-root"
      data-theme={preferences.theme}
      data-layout={preferences.layoutMode}
    >
      {chromeVisible && (
        <div className="reader-bar">
          <Link
            to={`/book/${workId}`}
            aria-label="← Library"
            className={cx(
              'inline-flex items-center gap-1.5 text-sm opacity-70 hover:opacity-100 transition-opacity',
              FOCUS_RING,
            )}
          >
            <ChevronLeftIcon className="size-4" aria-hidden="true" />
            <span>Library</span>
          </Link>
          <div className="min-w-0 flex-1 truncate text-center text-sm font-serif font-medium opacity-80">
            {bookQuery.data?.title}
          </div>
          <div className="flex flex-none items-center gap-2xs">
            <ToolButton
              active={panel === 'toc'}
              onClick={() => setPanel(panel === 'toc' ? null : 'toc')}
            >
              <BookOpenIcon className="size-3.5" aria-hidden="true" />
              <span>Contents</span>
            </ToolButton>
            <ToolButton
              active={panel === 'settings'}
              onClick={() => setPanel(panel === 'settings' ? null : 'settings')}
              label="Reading settings"
            >
              <span className="font-serif font-semibold text-xs tracking-tight" aria-hidden="true">
                Aa
              </span>
            </ToolButton>
            <ToolButton
              active={panel === 'marks'}
              onClick={() => setPanel(panel === 'marks' ? null : 'marks')}
            >
              <BookmarkIcon className="size-3.5" aria-hidden="true" />
              <span>Marks</span>
            </ToolButton>
            <ToolButton
              active={false}
              onClick={() => setChromeVisible(false)}
              label="Hide reading controls"
            >
              <MaximizeIcon className="size-3.5" aria-hidden="true" />
            </ToolButton>
          </div>
        </div>
      )}
      {!chromeVisible && (
        <button
          type="button"
          className={cx(
            'absolute right-md top-md z-10 rounded-md border bg-surface px-sm py-2xs text-xs',
            FOCUS_RING,
          )}
          onClick={() => setChromeVisible(true)}
        >
          Show controls
        </button>
      )}

      {grantFailed && wantedSrc !== frameWant && (
        <div className="reader-footer" role="alert">
          <span className="text-sm">This chapter couldn’t be loaded.</span>
          <button type="button" className={cx('text-sm', FOCUS_RING)} onClick={retryGrant}>
            Try again
          </button>
        </div>
      )}

      <iframe
        ref={iframeRef}
        className="reader-content-frame"
        title={`${bookQuery.data?.title ?? 'Book'} — reading area`}
        sandbox="allow-same-origin"
        src={frameSrc}
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
            {toc
              .map((entry, i) => ({
                entry,
                i,
                idx: sectionIndexForHref(sections, entry.href),
              }))
              .filter(({ idx }) => idx >= 0)
              .map(({ entry, i, idx }) => {
                const active = idx === sectionIndex
                return (
                  <li
                    key={`${entry.href}-${i}`}
                    style={{ paddingLeft: entry.depth ? '0.75rem' : undefined }}
                  >
                    <button
                      type="button"
                      className={cx('reader-toc-entry', FOCUS_RING)}
                      aria-current={active}
                      onClick={() => goToSection(idx)}
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
        <div
          className="reader-panel reader-panel--settings"
          role="group"
          aria-label="Reading settings"
        >
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
              Marks ·{' '}
              {(bookmarksQuery.data?.bookmarks.length ?? 0) +
                (highlightsQuery.data?.highlights.length ?? 0)}
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
            className={cx(
              'mt-md inline-flex w-full items-center justify-center gap-xs rounded-lg border p-sm text-sm font-medium transition-colors hover:bg-surface-2',
              FOCUS_RING,
            )}
            onClick={() => {
              const doc = iframeRef.current?.contentDocument ?? null
              if (currentSection) {
                void createBookmark.mutate({ cfi: positionCfi(currentSection, doc) })
              }
            }}
          >
            <BookmarkIcon className="size-4" aria-hidden="true" />
            <span>Bookmark this page</span>
          </button>

          <ul className="mt-md flex flex-col gap-sm">
            {bookmarksQuery.data?.bookmarks.map((b) => (
              <li key={b.id} className="rounded-lg border p-sm text-sm">
                <div className="flex items-center gap-xs">
                  <BookmarkIcon className="size-3.5 text-text" aria-hidden="true" />
                  <span className="font-mono text-2xs uppercase tracking-1 opacity-70">
                    Bookmark
                  </span>
                </div>
                <p className="mt-4xs">{b.label || 'Untitled bookmark'}</p>
              </li>
            ))}
            {highlightsQuery.data?.highlights.map((h) => (
              <li key={h.id} className="rounded-lg border p-sm text-sm">
                <div className="flex items-center gap-xs">
                  <span className="size-2 rounded-full bg-text-2" aria-hidden="true" />
                  <span className="font-mono text-2xs uppercase tracking-1 opacity-70">
                    Highlight
                  </span>
                </div>
                {h.note && <p className="mt-4xs opacity-80">{h.note}</p>}
              </li>
            ))}
            {(bookmarksQuery.data?.bookmarks.length ?? 0) === 0 &&
              (highlightsQuery.data?.highlights.length ?? 0) === 0 && (
                <li className="rounded-lg border border-dashed p-md text-center text-sm opacity-70">
                  <BookmarkIcon className="size-6 text-text-3 mx-auto mb-xs" aria-hidden="true" />
                  <p className="font-medium text-xs">Nothing marked yet</p>
                  <p className="mt-4xs text-2xs opacity-80">
                    Select a passage to highlight it, or bookmark this page. Marks are stored with
                    the library, not in this browser.
                  </p>
                </li>
              )}
          </ul>
        </aside>
      )}

      <div className="reader-footer">
        <button
          type="button"
          className={cx(
            'inline-flex items-center justify-center size-7 rounded-md text-sm opacity-70 hover:opacity-100 disabled:opacity-30 transition-opacity',
            FOCUS_RING,
          )}
          onClick={() => goToSection(sectionIndex - 1)}
          disabled={sectionIndex === 0}
          aria-label="Previous chapter"
        >
          <ChevronLeftIcon className="size-4" aria-hidden="true" />
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
        <div className="flex-none flex items-center gap-1 font-mono text-2xs opacity-70">
          <span>{Math.round(ratio * 100)}%</span>
          {Math.round(ratio * 100) === 100 && (
            <span
              className="inline-flex items-center text-3xs font-mono uppercase tracking-wide border border-border px-1 py-0.5 rounded text-text select-none"
              title="Book completed!"
              aria-label="Book completed"
            >
              Done
            </span>
          )}
        </div>
        <button
          type="button"
          className={cx(
            'inline-flex items-center justify-center size-7 rounded-md text-sm opacity-70 hover:opacity-100 disabled:opacity-30 transition-opacity',
            FOCUS_RING,
          )}
          onClick={() => goToSection(sectionIndex + 1)}
          disabled={sectionIndex === sections.length - 1}
          aria-label="Next chapter"
        >
          <ChevronRightIcon className="size-4" aria-hidden="true" />
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
        'inline-flex items-center gap-xs h-8 rounded-md border px-sm text-xs font-medium transition-colors hover:bg-surface-2',
        active && 'bg-surface-2',
        FOCUS_RING,
      )}
    >
      {children}
    </button>
  )
}

/** The spine index to open at, from a saved ReadingProgress. */
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
