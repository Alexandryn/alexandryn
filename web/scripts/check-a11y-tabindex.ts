import { reportFindingsAndExit, requireDir } from './checks/cli.ts'
import { findPositiveTabindex } from './checks/positiveTabindex.ts'

// One or more source roots (default: src, e2e). frontend-accessibility.md
// FR-2 / Acceptance criterion 3.
const dirs = process.argv.slice(2)
if (dirs.length === 0) dirs.push('src', 'e2e')

const findings = dirs.flatMap((dir) => {
  requireDir(dir, 'check-a11y-tabindex')
  return findPositiveTabindex(dir)
})

reportFindingsAndExit(
  'check-a11y-tabindex',
  findings,
  'positive tabindex found (frontend-accessibility.md FR-2 — focus order must follow DOM order):',
)
