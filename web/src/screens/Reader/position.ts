import * as CFI from '../../vendor/foliate/epubcfi'
import type { EpubSection } from '../../vendor/foliate/epub'

/**
 * Builds a CFI for the current reading position: the section's own
 * spine-step CFI (foliate-generated) joined with a local CFI derived
 * from a Range at the first block element visible in the viewport
 * (foliate's `fromRange`, never hand-derived). Falls back to the
 * bare spine-step CFI if a local Range can't be formed.
 */
export function positionCfi(section: EpubSection, doc: Document | null): string {
  if (!doc) return wrap(section.cfi)
  try {
    const el = firstVisibleBlock(doc)
    if (el) {
      const range = doc.createRange()
      range.selectNode(el)
      const local = CFI.fromRange(range)
      return CFI.joinIndir(wrap(section.cfi), local)
    }
  } catch {
    // fall through to the spine-step CFI
  }
  return wrap(section.cfi)
}

function wrap(step: string): string {
  return CFI.isCFI.test(step) ? step : `epubcfi(${step})`
}

/**
 * The start and end CFIs of a text selection, each the section's spine-step
 * CFI joined with a local CFI for that endpoint of the user's `Range`
 * (foliate's `fromRange` on a collapsed range — never hand-derived).
 * Earlier code reused the first-visible-block position for both ends, so
 * every highlight was stored zero-length. Falls back to
 * a single position when the range can't be read.
 */
export function selectionCfis(
  section: EpubSection,
  doc: Document,
  range: Range,
): { start: string; end: string } {
  const step = wrap(section.cfi)
  try {
    const endpoint = (container: Node, offset: number): string => {
      const r = doc.createRange()
      r.setStart(container, offset)
      r.collapse(true)
      return CFI.joinIndir(step, CFI.fromRange(r))
    }
    return {
      start: endpoint(range.startContainer, range.startOffset),
      end: endpoint(range.endContainer, range.endOffset),
    }
  } catch {
    const fallback = positionCfi(section, doc)
    return { start: fallback, end: fallback }
  }
}

function firstVisibleBlock(doc: Document): Element | null {
  const candidates = doc.body?.querySelectorAll('p, h1, h2, h3, h4, h5, h6, li, blockquote, div')
  if (!candidates) return null
  for (const el of Array.from(candidates)) {
    const rect = el.getBoundingClientRect()
    if (rect.height > 0 && rect.bottom > 0) return el
  }
  return doc.body?.firstElementChild ?? null
}

/**
 * The spine-step part of a stored CFI (everything before the first `!`),
 * used to find which section a saved position lives in.
 */
export function spineStepOf(cfi: string): string {
  const m = cfi.match(/^epubcfi\(([^!]+)/)
  return m && m[1] ? `epubcfi(${m[1].trim()})` : cfi
}

/** The section index a stored CFI points into, or -1. */
export function sectionIndexForCfi(sections: EpubSection[], cfi: string): number {
  const step = spineStepOf(cfi)
  return sections.findIndex((s) => wrap(s.cfi) === step)
}

/** A deterministic progression fraction from spine index + scroll. */
export function progressRatio(index: number, count: number, scrollFraction: number): number {
  if (count <= 0) return 0
  const clamped = Math.min(1, Math.max(0, scrollFraction))
  return Math.min(1, (index + clamped) / count)
}

/**
 * Inverts {@link progressRatio}: given a saved book-wide percentage and the
 * section it resolves to, the scroll offset within that section (0..1).
 * Restores the reader to where the reader actually stopped, not just the
 * top of the chapter. Clamped, because a CFI may
 * pin a different section than a stale percentage implies.
 */
export function sectionScrollFraction(percentage: number, index: number, count: number): number {
  if (count <= 0) return 0
  return Math.min(1, Math.max(0, percentage * count - index))
}

/**
 * Scrolls a chapter document to `fraction` (0..1) of its scrollable
 * height. `fraction <= 0` scrolls to the top — the common case for
 * chapter-to-chapter navigation. The layout-dependent branch needs a real
 * browser to exercise; jsdom reports zero heights.
 */
export function restoreScroll(
  win: Pick<Window, 'scrollTo'> & {
    document: Pick<Document, 'scrollingElement' | 'documentElement'>
  },
  fraction: number,
): void {
  if (fraction <= 0) {
    win.scrollTo(0, 0)
    return
  }
  const el = win.document.scrollingElement ?? win.document.documentElement
  const denom = el.scrollHeight - el.clientHeight
  win.scrollTo(0, denom > 0 ? denom * fraction : 0)
}
