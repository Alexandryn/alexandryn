import { join } from 'node:path'
import { readFileSync } from 'node:fs'
import { gzipSync } from 'node:zlib'
import { requireDir } from './checks/cli.ts'
import {
  BUNDLE_SIZE_BUDGET_BYTES,
  ENTRY_CHUNK_BUDGET_BYTES,
  findEntryChunk,
  measureJsGzipBytes,
} from './checks/bundleSize.ts'

const distDir = process.argv[2] ?? 'dist'
requireDir(join(distDir, 'assets'), 'check-bundle-size')

// Gate 1: total JS budget (all chunks combined).
const totalBytes = measureJsGzipBytes(distDir)
const totalBudgetKiB = (BUNDLE_SIZE_BUDGET_BYTES / 1024).toFixed(0)
const totalActualKiB = (totalBytes / 1024).toFixed(1)

let failed = false

if (totalBytes > BUNDLE_SIZE_BUDGET_BYTES) {
  console.error(
    `check-bundle-size: total JS ${totalActualKiB} KiB gzipped exceeds the ${totalBudgetKiB} KiB budget`,
  )
  failed = true
} else {
  console.log(`check-bundle-size: total JS ${totalActualKiB} KiB gzipped, within the ${totalBudgetKiB} KiB budget`)
}

// Gate 2: entry-chunk budget. The entry chunk is the
// initial payload — lazy routes must not silently collapse back into it.
// If the entry chunk cannot be identified (Vite version/naming change),
// warn rather than hard-fail so the check degrades gracefully.
const entryChunkPath = findEntryChunk(distDir)
if (entryChunkPath === null) {
  console.warn(
    'check-bundle-size: could not identify entry chunk (no index-*.js in dist/assets) — entry-chunk budget not checked',
  )
} else {
  const entryBytes = gzipSync(readFileSync(entryChunkPath)).length
  const entryBudgetKiB = (ENTRY_CHUNK_BUDGET_BYTES / 1024).toFixed(0)
  const entryActualKiB = (entryBytes / 1024).toFixed(1)
  if (entryBytes > ENTRY_CHUNK_BUDGET_BYTES) {
    console.error(
      `check-bundle-size: entry chunk ${entryActualKiB} KiB gzipped exceeds the ${entryBudgetKiB} KiB budget — lazy routes may have collapsed back into the initial load`,
    )
    failed = true
  } else {
    console.log(
      `check-bundle-size: entry chunk ${entryActualKiB} KiB gzipped, within the ${entryBudgetKiB} KiB budget`,
    )
  }
}

if (failed) process.exit(1)
