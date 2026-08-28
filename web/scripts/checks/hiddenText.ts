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
// Scans .ts/.tsx source only: Tailwind classes / CSS-in-JS live there.
// This project has no per-component .css files — only the generated token
// files and the hand-authored a11y.css, neither of which hides text.
//
// A genuinely non-text use (collapsing a layout region with nothing to
// announce) opts out with `// a11y-hidden-text-ok: <reason>` on that line
// or the one above it.
const DIRECT_PATTERNS: { name: string; re: RegExp }[] = [
  { name: 'display:none', re: /display\s*:\s*['"]?none/i },
  { name: 'clip:rect', re: /clip\s*:\s*rect\(/i },
  { name: 'clip-path:inset(50%)', re: /clip-?path\s*:\s*['"]?inset\(\s*50%/i },
]

// The classic hand-rolled sr-only: a 1px box that is also clipped. Each
// part alone is a legitimate hairline / clipped container — flagged only
// when they appear together on one line (order-independent, since Tailwind
// class order is arbitrary).
const CLUSTER = [/\bw-px\b/, /\bh-px\b/, /\boverflow-hidden\b/]

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
      if (ESCAPE_HATCH.test(line) || (i > 0 && ESCAPE_HATCH.test(lines[i - 1] ?? ''))) return

      for (const { name: patternName, re } of DIRECT_PATTERNS) {
        if (re.test(line)) findings.push({ file, pattern: patternName })
      }
      if (CLUSTER.every((re) => re.test(line))) {
        findings.push({
          file,
          pattern: 'hand-rolled sr-only cluster (w-px + h-px + overflow-hidden)',
        })
      }
    })
  }
  return findings
}
