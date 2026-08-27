import { readFileSync } from 'node:fs'
import { walkSourceFiles } from './walkSrc.ts'

// Interim, grep-based (frontend-component-primitives.md FR-4/Acceptance
// criteria) until a lint rule exists. Catches the two shapes a raw value
// takes in this codebase: a literal hex color, and a literal pixel value
// inside an inline style or a Tailwind arbitrary-value bracket.
const RAW_VALUE_PATTERNS: RegExp[] = [
  /#(?:[0-9a-fA-F]{3}){1,2}\b/, // hex color, e.g. #fff, #1a1917
  /\[-?\d+(?:\.\d+)?px\]/, // Tailwind arbitrary bracket value, e.g. w-[16px]
  /:\s*['"`]-?\d+(?:\.\d+)?px['"`]/, // inline style value, e.g. width: '16px'
]

export interface TokenStylingFinding {
  file: string
  pattern: string
}

/** Scans every .ts/.tsx file under componentsDir for a raw hex/px value outside the token set. */
export function findRawStyleValues(componentsDir: string): TokenStylingFinding[] {
  const findings: TokenStylingFinding[] = []
  for (const file of walkSourceFiles(componentsDir)) {
    const contents = readFileSync(file, 'utf8')
    for (const pattern of RAW_VALUE_PATTERNS) {
      if (pattern.test(contents)) {
        findings.push({ file, pattern: pattern.source })
      }
    }
  }
  return findings
}
