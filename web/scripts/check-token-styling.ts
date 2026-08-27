import { requireDir, reportFindingsAndExit } from './checks/cli.ts'
import { findRawStyleValues } from './checks/tokenStyling.ts'

// One or more source roots (default: src/components). Tier 4 adds src/app
// and src/screens so the shell and route views are held to the same
// token-only bar as the primitives (frontend-component-primitives.md
// FR-4, frontend-shell-and-routing.md's own token-only styling).
const dirs = process.argv.slice(2)
if (dirs.length === 0) dirs.push('src/components')

const findings = dirs.flatMap((dir) => {
  requireDir(dir, 'check-token-styling')
  return findRawStyleValues(dir)
})

reportFindingsAndExit(
  'check-token-styling',
  findings,
  'raw hex/px values found outside the token set (frontend-component-primitives.md FR-4):',
)
