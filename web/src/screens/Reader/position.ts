import * as CFI from '../../vendor/foliate/epubcfi'
import type { EpubSection } from '../../vendor/foliate/epub'

/**
 * Builds a CFI for the current reading position: the section's own
 * spine-step CFI (foliate-generated) joined with a local CFI derived
 * from a Range at the first block element visible in the viewport
 * (foliate's `fromRange`, never hand-derived — FR-5). Falls back to the
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
