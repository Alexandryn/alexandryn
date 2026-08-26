import { requireDir, reportFindingsAndExit } from './checks/cli.ts'
import { findMswReferences } from './checks/mswExclusion.ts'

const distDir = process.argv[2] ?? 'dist'
requireDir(distDir, 'check-dist-msw')

reportFindingsAndExit(
  'check-dist-msw',
  findMswReferences(distDir),
  'MSW reference found in the production bundle:',
)
