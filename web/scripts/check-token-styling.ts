import { requireDir, reportFindingsAndExit } from './checks/cli.ts'
import { findRawStyleValues } from './checks/tokenStyling.ts'

const componentsDir = process.argv[2] ?? 'src/components'
requireDir(componentsDir, 'check-token-styling')

reportFindingsAndExit(
  'check-token-styling',
  findRawStyleValues(componentsDir),
  'raw hex/px values found outside the token set (frontend-component-primitives.md FR-4):',
)
