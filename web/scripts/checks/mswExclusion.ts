import { readFileSync } from 'node:fs'
import { basename } from 'node:path'
import { walkScannableFiles } from './walkDist.ts'

// Production exclusion check: ensures MSW mock worker and code
// are not bundled into production builds.
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
// Verified against a real MSW 2.15 build in Tier 4: MSW's own generated
// worker script does NOT contain the string "mockServiceWorker" in its
// body, so a content grep alone would miss the file itself if it were
// ever copied into dist. The check therefore has two arms — a content
// pattern (catches `worker.start()`-style references that survive
// bundling) and a filename match (catches the worker script by name,
// regardless of contents).
const MSW_PATTERN = /mockServiceWorker/
const MSW_WORKER_FILENAME = 'mockServiceWorker.js'

export interface MswFinding {
  file: string
  pattern: string
}

/** Scans every scannable file in distDir (the whole tree) for MSW references. */
export function findMswReferences(distDir: string): MswFinding[] {
  const findings: MswFinding[] = []
  for (const file of walkScannableFiles(distDir)) {
    if (basename(file) === MSW_WORKER_FILENAME) {
      findings.push({ file, pattern: `filename: ${MSW_WORKER_FILENAME}` })
      continue
    }
    const contents = readFileSync(file, 'utf8')
    if (MSW_PATTERN.test(contents)) {
      findings.push({ file, pattern: MSW_PATTERN.source })
    }
  }
  return findings
}
