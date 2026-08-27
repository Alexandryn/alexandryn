import { reportFindingsAndExit, requireDir } from './checks/cli.ts'
import { findHandRolledHiddenText } from './checks/hiddenText.ts'

// One or more source roots (default: src/components, src/app, src/screens).
// frontend-accessibility.md FR-3 / Acceptance criterion 4.
const dirs = process.argv.slice(2)
if (dirs.length === 0) dirs.push('src/components', 'src/app', 'src/screens')

const findings = dirs.flatMap((dir) => {
  requireDir(dir, 'check-a11y-hidden-text')
  return findHandRolledHiddenText(dir)
})

reportFindingsAndExit(
  'check-a11y-hidden-text',
  findings,
  'hand-rolled hidden text found (frontend-accessibility.md FR-3 — use <VisuallyHidden> or sr-only):',
)
