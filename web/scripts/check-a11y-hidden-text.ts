import { reportFindingsAndExit, requireDir } from './checks/cli.ts'
import { findHandRolledHiddenText } from './checks/hiddenText.ts'

// One or more source roots (default: all of src — convention is
// "anywhere", and a helper in src/lib or src/data can render nodes too).
const dirs = process.argv.slice(2)
if (dirs.length === 0) dirs.push('src')

const findings = dirs.flatMap((dir) => {
  requireDir(dir, 'check-a11y-hidden-text')
  return findHandRolledHiddenText(dir)
})

reportFindingsAndExit(
  'check-a11y-hidden-text',
  findings,
  'hand-rolled hidden text found (use <VisuallyHidden> or sr-only):',
)
