import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

// frontend-shell-and-routing.md FR-6 requires MSW for development/
// testing; this is the production-exclusion check that requirement
// depends on. Matches an actual import specifier for the msw package or
// its generated service-worker filename — not a bare "msw" substring,
// which would false-positive on unrelated identifiers.
const MSW_PATTERNS: RegExp[] = [
  /from\s*["']msw(\/|["'])/,
  /require\(\s*["']msw(\/|["'])/,
  /mockServiceWorker/,
]

export interface MswFinding {
  file: string
  pattern: string
}

/** Scans every .js file under distDir/assets for MSW references. */
export function findMswReferences(distDir: string): MswFinding[] {
  const assetsDir = join(distDir, 'assets')
  const findings: MswFinding[] = []
  for (const name of readdirSync(assetsDir)) {
    if (!name.endsWith('.js')) continue
    const contents = readFileSync(join(assetsDir, name), 'utf8')
    for (const pattern of MSW_PATTERNS) {
      if (pattern.test(contents)) {
        findings.push({ file: join(assetsDir, name), pattern: pattern.source })
      }
    }
  }
  return findings
}
