import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { extractTokens } from './tokens/extract.ts'
import { contrastRatio, meetsWcagAA } from './tokens/contrast.ts'

const DESIGN_REFERENCE_DIR = join(import.meta.dirname, '..', '..', '.design-reference')
const CANVASES: Record<string, string> = {
  electron: 'Alexandryn-Electron.dc.html',
  admin: 'Alexandryn-Electron-Admin.dc.html',
  web: 'Alexandryn-Web.dc.html',
  mobile: 'Alexandryn-Mobile.dc.html',
}

// Every text-shaped token against every surface it could plausibly render
// on — the meaningful subset of "every color token pair"
// (frontend-design-tokens.md's own Test strategy), not the full
// combinatorial cross-product of unrelated tokens (e.g. border vs warm,
// which never appear as a text/background relationship).
const TEXT_TOKENS = ['text', 'text-2', 'text-3']
const SURFACE_TOKENS = ['background', 'surface', 'surface-2', 'surface-3']
const EXTRA_PAIRS: [string, string][] = [['accent-text', 'accent']]

// Known, accepted exception — NOT a silent fix (frontend-design-tokens.md
// FR-4 forbids inventing a corrected value). text-3 (#9C978F) fails WCAG
// AA against every surface in the current design reference — 2.90:1 at
// best, below even the large-text 3:1 bar. Recorded here rather than
// either (a) silently passing the check or (b) blocking Tier 1's
// otherwise-correct extraction pipeline on an open design question.
// Needs a maintainer decision: is text-3 decorative/non-text-conveying
// only (in which case this exception is permanent), or does the design
// reference need a darker tertiary text value (in which case this
// exception is removed once that lands)? Not decided here.
//
// Deliberately a fixed literal list of the four exact pairs actually
// measured — not derived from SURFACE_TOKENS.map(...) (code review
// caught this: a derived set would silently start covering any new
// surface token added later, e.g. "text-3 on surface-4", the moment
// SURFACE_TOKENS changes, without that specific ratio ever having been
// measured or confirmed to be the same known issue rather than a new,
// unrelated regression).
const KNOWN_EXCEPTIONS = new Set([
  'text-3 on background',
  'text-3 on surface',
  'text-3 on surface-2',
  'text-3 on surface-3',
])

const canvasHtml: Record<string, string> = {}
for (const [name, file] of Object.entries(CANVASES)) {
  canvasHtml[name] = readFileSync(join(DESIGN_REFERENCE_DIR, file), 'utf8')
}
const tokens = extractTokens(canvasHtml)
const colorByName = new Map(tokens.colors.map((c) => [c.name, c.value]))

const pairs: [string, string][] = [
  ...TEXT_TOKENS.flatMap((t) => SURFACE_TOKENS.map((s): [string, string] => [t, s])),
  ...EXTRA_PAIRS,
]

let failed = false
for (const [fg, bg] of pairs) {
  const fgValue = colorByName.get(fg)
  const bgValue = colorByName.get(bg)
  if (!fgValue || !bgValue) {
    console.error(`check-token-contrast: unknown token in pair "${fg} on ${bg}"`)
    process.exit(1)
  }
  const ratio = contrastRatio(fgValue, bgValue)
  const pass = meetsWcagAA(fgValue, bgValue)
  const label = `${fg} on ${bg}`
  const isKnownException = KNOWN_EXCEPTIONS.has(label)

  if (pass) {
    console.log(`  PASS  ${label.padEnd(24)} ${ratio.toFixed(2)}:1`)
  } else if (isKnownException) {
    console.log(
      `  FAIL  ${label.padEnd(24)} ${ratio.toFixed(2)}:1  (known exception, see script comment)`,
    )
  } else {
    console.error(`  FAIL  ${label.padEnd(24)} ${ratio.toFixed(2)}:1`)
    failed = true
  }
}

// The prefers-contrast: more fallback (frontend-accessibility.md FR-5):
// src/a11y.css must redefine --color-text-3 to a value that passes AA on
// every surface, so the KNOWN_EXCEPTIONS above are mitigated for a
// high-contrast user rather than left flat.
const a11yCss = readFileSync(join(import.meta.dirname, '..', 'src', 'a11y.css'), 'utf8')
const overrideMatch =
  /@media\s*\(prefers-contrast:\s*more\)\s*\{[\s\S]*?--color-text-3:\s*var\(--color-([a-z0-9-]+)\)/i.exec(
    a11yCss,
  )
if (!overrideMatch) {
  console.error(
    'check-token-contrast: src/a11y.css is missing the prefers-contrast: more override for --color-text-3 (frontend-accessibility.md FR-5)',
  )
  process.exit(1)
}
const fallbackValue = colorByName.get(overrideMatch[1] as string)
if (!fallbackValue) {
  console.error(
    `check-token-contrast: a11y.css text-3 fallback references unknown token --color-${overrideMatch[1]}`,
  )
  process.exit(1)
}
for (const surface of SURFACE_TOKENS) {
  const bgValue = colorByName.get(surface) as string
  const ratio = contrastRatio(fallbackValue, bgValue)
  if (meetsWcagAA(fallbackValue, bgValue)) {
    console.log(`  PASS  text-3 (prefers-contrast) on ${surface.padEnd(11)} ${ratio.toFixed(2)}:1`)
  } else {
    console.error(
      `  FAIL  text-3 (prefers-contrast) on ${surface} ${ratio.toFixed(2)}:1 — the FR-5 fallback must pass AA`,
    )
    failed = true
  }
}

if (failed) {
  console.error(
    'check-token-contrast: one or more pairs fail WCAG AA (4.5:1) with no recorded exception',
  )
  process.exit(1)
}

console.log('check-token-contrast: clean (all pairs pass, or are a recorded, documented exception)')
