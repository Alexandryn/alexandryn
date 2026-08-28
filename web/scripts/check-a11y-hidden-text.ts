import { reportFindingsAndExit, requireDir } from './checks/cli.ts'
import { findHandRolledHiddenText } from './checks/hiddenText.ts'

// One or more source roots (default: all of src — FR-3's convention is
// "anywhere", and a helper in src/lib or src/data can render nodes too).
// frontend-accessibility.md FR-3 / Acceptance criterion 4.
const dirs = process.argv.slice(2)
if (dirs.length === 0) dirs.push('src')

const findings = dirs.flatMap((dir) => {
  requireDir(dir, 'check-a11y-hidden-text')
  return findHandRolledHiddenText(dir)
})

reportFindingsAndExit(
  'check-a11y-hidden-text',
  findings,
  'hand-rolled hidden text found (frontend-accessibility.md FR-3 — use <VisuallyHidden> or sr-only):',
)
