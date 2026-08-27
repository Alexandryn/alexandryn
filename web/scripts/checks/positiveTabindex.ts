import { readFileSync } from 'node:fs'
import { walkFilesByExtension } from './walkFiles.ts'

// frontend-accessibility.md FR-2: focus order follows DOM order — a
// positive tabindex reorders it and is never allowed. `tabIndex={-1}`
// (remove from the sequence) and `{0}` (keep in natural order) are fine.
// Interim grep-based check until a lint rule covers it; jsx-a11y's
// `tabindex-no-positive` exists but isn't in this project's enabled set.
const SCANNABLE = new Set(['.ts', '.tsx', '.html'])

// tabIndex={1}, tabIndex={"2"}, tabindex="3" — a leading digit 1-9 means positive.
const PATTERNS: RegExp[] = [
  /tabIndex=\{\s*['"]?[1-9]\d*['"]?\s*\}/,
  /tabindex\s*=\s*["'][1-9]\d*["']/i,
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
