import { readFileSync } from 'node:fs'
import { walkFilesByExtension } from './walkFiles.ts'

// frontend-accessibility.md FR-2: focus order follows DOM order — a
// positive tabindex reorders it and is never allowed. `tabIndex={-1}`
// (remove from the sequence) and `{0}` (keep in natural order) are fine.
//
// `eslint-plugin-jsx-a11y/tabindex-no-positive` is enabled and catches a
// literal `tabIndex={2}`, but not a conditional like `tabIndex={x ? 2 : 0}`
// and not `.html` attributes. This grep covers both — the spec's
// Acceptance criterion 3 asks for a grep-based check specifically — and
// runs over all of src plus e2e.
const SCANNABLE = new Set(['.ts', '.tsx', '.html'])

const PATTERNS: RegExp[] = [
  // tabIndex={1}, tabIndex={"2"}
  /tabIndex=\{\s*['"]?[1-9]\d*['"]?\s*\}/,
  // tabindex="3" (HTML / DOM attribute)
  /tabindex\s*=\s*["'][1-9]\d*["']/i,
  // A positive integer literal in a ternary value position inside a
  // tabIndex expression: `? 2 :`, `: 2 }`, `: 2)`. `? 0`, `: -1` and a
  // comparison like `=== 2` are not matched.
  /tabIndex=\{[^}]*[?:]\s*[1-9]\d*\s*[\s:})]/,
]

export interface TabindexFinding {
  file: string
  pattern: string
}

export function findPositiveTabindex(dir: string): TabindexFinding[] {
  const findings: TabindexFinding[] = []
  for (const file of walkFilesByExtension(dir, SCANNABLE)) {
    const contents = readFileSync(file, 'utf8')
    for (const pattern of PATTERNS) {
      if (pattern.test(contents)) {
        findings.push({ file, pattern: pattern.source })
      }
    }
  }
  return findings
}
