import { requireDir, reportFindingsAndExit } from './checks/cli.ts'
import { findRawStyleValues } from './checks/tokenStyling.ts'

// One or more source roots (default: src/components). Includes src/app
// and src/screens so the shell and route views are held to the same
// token-only bar as the primitives.
const dirs = process.argv.slice(2)
if (dirs.length === 0) dirs.push('src/components')

const findings = dirs.flatMap((dir) => {
  requireDir(dir, 'check-token-styling')
  return findRawStyleValues(dir)
})

reportFindingsAndExit(
  'check-token-styling',
  findings,
  'raw hex/px values found outside the token set:',
)
