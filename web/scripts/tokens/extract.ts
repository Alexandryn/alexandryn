import { COLOR_NAMES, SHADOW_NAMES, SIZE_NAMES } from './colorNames.ts'
import { deriveScale, type ScaleToken } from './deriveScale.ts'
import { type BreakpointToken, extractBreakpoint } from './extractBreakpoint.ts'
import { mergeCanvasProperties, type MergedProperty } from './mergeCanvasProperties.ts'
import { countEmValues, countPxValues, parseRootCustomProperties } from './parseCanvas.ts'

// Thresholds are a documented, fixed rule, not tuned per category to hit
// a "nicer-looking" token count — invention of values is forbidden, and
// an ad hoc threshold-per-category would be the
// same failure mode one step removed. 15 for px-based properties
// (radius, spacing) — high enough to exclude clear one-off noise (the
// design reference is a hand-tuned prototype, not a systematic grid;
// aggregated counts range into the hundreds for common values and down
// to 1 for rare ones), low enough not to discard real, recurring
// choices. 3 for letter-spacing specifically: it's used far more
// sparingly overall (dozens of occurrences total, not hundreds), so the
// same threshold would keep almost nothing.
const PX_THRESHOLD = 15
const EM_THRESHOLD = 3

export interface NamedColor {
  name: string
  varName: string
  value: string
  canvases: string[]
}

export interface LetterSpacingToken {
  name: string
  em: string
  count: number
  isDefault: boolean
}

export interface ExtractedTokens {
  colors: NamedColor[]
  shadows: NamedColor[]
  sizes: NamedColor[]
  radius: ScaleToken[]
  spacing: ScaleToken[]
  fontSize: ScaleToken[]
  letterSpacing: LetterSpacingToken[]
  breakpoint: BreakpointToken
}

function nameMerged(
  merged: Record<string, MergedProperty>,
  names: Record<string, string>,
): NamedColor[] {
  return Object.entries(names)
    .filter(([varName]) => merged[varName])
    .map(([varName, name]) => {
      const m = merged[varName] as MergedProperty
      return { name, varName, value: m.value, canvases: m.canvases }
    })
}

function sumCounts(maps: Record<number, number>[]): Record<number, number> {
  const total: Record<number, number> = {}
  for (const map of maps) {
    for (const [k, v] of Object.entries(map)) {
      const key = Number(k)
      total[key] = (total[key] ?? 0) + v
    }
  }
  return total
}

/** Extracts every token category from the four design-reference canvases. */
export function extractTokens(canvasHtmlByName: Record<string, string>): ExtractedTokens {
  const parsed: Record<string, Record<string, string>> = {}
  for (const [name, html] of Object.entries(canvasHtmlByName)) {
    parsed[name] = parseRootCustomProperties(html)
  }
  const merged = mergeCanvasProperties(parsed)

  // Loud, not silent, if the design reference ever adds a root custom
  // property this pipeline doesn't know how to name — the same
  // discipline mergeCanvasProperties already applies to a value
  // disagreement. Code review caught that nameMerged alone just drops
  // anything not already in COLOR_NAMES/SHADOW_NAMES/SIZE_NAMES, so a
  // future canvas update could silently vanish from every output
  // category with nothing catching it — no thrown error, no CI failure.
  const known = new Set([
    ...Object.keys(COLOR_NAMES),
    ...Object.keys(SHADOW_NAMES),
    ...Object.keys(SIZE_NAMES),
  ])
  const unknown = Object.keys(merged).filter((varName) => !known.has(varName))
  if (unknown.length > 0) {
    throw new Error(
      `extractTokens: unrecognized custom propert${unknown.length === 1 ? 'y' : 'ies'} ${unknown.join(', ')} — add to COLOR_NAMES/SHADOW_NAMES/SIZE_NAMES in colorNames.ts before extracting`,
    )
  }

  const htmls = Object.values(canvasHtmlByName)
  const radiusCounts = sumCounts(htmls.map((h) => countPxValues(h, 'border-radius')))
  const spacingCounts = sumCounts(htmls.map((h) => countPxValues(h, 'gap')))
  const fontSizeCounts = sumCounts(htmls.map((h) => countPxValues(h, 'font-size')))

  const letterSpacingCounts: Record<string, number> = {}
  for (const h of htmls) {
    for (const [em, count] of Object.entries(countEmValues(h, 'letter-spacing'))) {
      letterSpacingCounts[em] = (letterSpacingCounts[em] ?? 0) + count
    }
  }
  const keptLetterSpacing = Object.entries(letterSpacingCounts)
    .filter(([, count]) => count >= EM_THRESHOLD)
    .map(([em, count]) => ({ em, count }))
    .sort((a, b) => Number(a.em) - Number(b.em))
  // Named by ascending tracking value (tracking-1 = tightest ... tracking-N
  // = widest), not by a "tighter/normal/wider" label scheme — the most-
  // frequent real value is marked isDefault instead of assuming list
  // position maps to "normal" tracking, which broke on real data (the
  // actual most-used value, .1em/56 occurrences, isn't list-centered:
  // only 4 of the 11 kept values are negative).
  const maxCount = Math.max(...keptLetterSpacing.map((t) => t.count))
  const letterSpacing: LetterSpacingToken[] = keptLetterSpacing.map((t, i) => ({
    ...t,
    name: String(i + 1),
    isDefault: t.count === maxCount,
  }))

  return {
    colors: nameMerged(merged, COLOR_NAMES),
    shadows: nameMerged(merged, SHADOW_NAMES),
    sizes: nameMerged(merged, SIZE_NAMES),
    radius: deriveScale(radiusCounts, PX_THRESHOLD),
    spacing: deriveScale(spacingCounts, PX_THRESHOLD),
    fontSize: deriveScale(fontSizeCounts, PX_THRESHOLD),
    letterSpacing,
    breakpoint: extractBreakpoint(canvasHtmlByName),
  }
}
