import { readFileSync } from 'node:fs'
import { basename } from 'node:path'
import { walkSourceFiles } from './walkSrc.ts'

// frontend-accessibility.md FR-3: screen-reader-only text uses the shared
// <VisuallyHidden> primitive (or Tailwind's own `sr-only`), never
// `display: none` (which removes it from the accessibility tree too) or a
// per-component hand-rolled clip-rect. axe can't reliably catch this —
// the text is correctly hidden from a sighted user either way; the defect
// is that it's *also* gone from the a11y tree — so a grep is the check
// (Failure modes table).
//
// A genuinely non-text use of `display:none` (collapsing a layout region
// with nothing to announce) opts out with a line comment:
//   // a11y-hidden-text-ok: <reason>
const PATTERNS: { name: string; re: RegExp }[] = [
  { name: 'display:none', re: /display\s*:\s*['"]?none/i },
  { name: 'clip:rect', re: /clip\s*:\s*rect\(/i },
  { name: 'clip-path:inset(50%)', re: /clip-?path\s*:\s*['"]?inset\(\s*50%/i },
  // The classic hand-rolled sr-only: a 1px box, positioned, clipped.
  { name: 'hand-rolled sr-only cluster', re: /\bw-px\b[^"'`]*\bh-px\b[^"'`]*\boverflow-hidden\b/ },
]

const ESCAPE_HATCH = /a11y-hidden-text-ok:/

export interface HiddenTextFinding {
  file: string
  pattern: string
}

export function findHandRolledHiddenText(dir: string): HiddenTextFinding[] {
  const findings: HiddenTextFinding[] = []
  for (const file of walkSourceFiles(dir)) {
    const name = basename(file)
    if (/\.(test|stories)\.tsx?$/.test(name)) continue
    // The primitive itself is the one sanctioned implementation.
    if (name === 'VisuallyHidden.tsx') continue

    const lines = readFileSync(file, 'utf8').split('\n')
    lines.forEach((line, i) => {
      if (line.includes('sr-only') || ESCAPE_HATCH.test(line)) return
      // Allow the escape hatch on the immediately preceding line too.
      if (i > 0 && ESCAPE_HATCH.test(lines[i - 1] ?? '')) return
      for (const { name: patternName, re } of PATTERNS) {
        if (re.test(line)) findings.push({ file, pattern: patternName })
      }
    })
  }
  return findings
}
