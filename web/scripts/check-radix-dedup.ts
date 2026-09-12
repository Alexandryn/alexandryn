import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { findRadixVersionSplits } from './checks/radixDedup.ts'

// The lockfile is at the workspace root, one level up from web/.
const lockfilePath = process.argv[2] ?? join(process.cwd(), '..', 'package-lock.json')

if (!existsSync(lockfilePath)) {
  console.error(`check-radix-dedup: ${lockfilePath} does not exist`)
  process.exit(1)
}

const splits = findRadixVersionSplits(lockfilePath)

if (splits.length > 0) {
  console.error(
    'check-radix-dedup: @radix-ui packages resolve to more than one version:',
  )
  for (const s of splits) {
    console.error(`  ${s.name}: ${s.versions.join(', ')}`)
  }
  console.error(
    '  Align the @radix-ui/* ranges in web/package.json and run "npm dedupe --scope @radix-ui".',
  )
  process.exit(1)
}

console.log('check-radix-dedup: every @radix-ui package resolves to a single version — clean')
