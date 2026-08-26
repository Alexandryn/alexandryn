import { readFileSync } from 'node:fs'
import { walkScannableFiles } from './walkDist.ts'

// frontend-shell-and-routing.md FR-6 requires MSW for development/
// testing; this is the production-exclusion check that requirement
// depends on.
//
// Only `mockServiceWorker` is checked, deliberately — an earlier version
// of this file also matched `from "msw/..."`/`require("msw...")` import
// syntax, but code review confirmed empirically that Rollup's production
// bundling erases that syntax entirely (inlines the module body, no
// trace of the specifier string survives), so those patterns could never
// fire against a real build. `mockServiceWorker` is the name of MSW's
// own generated worker script, which it must reference literally
// (as a string, to register/fetch it) wherever it's wired up — the one
// marker that actually has a chance of surviving bundling.
//
// Not yet proven against a real MSW-containing production build (MSW
// isn't installed until Tier 4, frontend-shell-and-routing.md FR-6) —
// Tier 4 must re-verify this check actually catches a real leak once
// MSW exists in this repo, not just the synthetic fixture below.
const MSW_PATTERN = /mockServiceWorker/

export interface MswFinding {
  file: string
  pattern: string
}

/** Scans every scannable file in distDir (the whole tree) for MSW references. */
export function findMswReferences(distDir: string): MswFinding[] {
  const findings: MswFinding[] = []
  for (const file of walkScannableFiles(distDir)) {
    const contents = readFileSync(file, 'utf8')
    if (MSW_PATTERN.test(contents)) {
      findings.push({ file, pattern: MSW_PATTERN.source })
    }
  }
  return findings
}
