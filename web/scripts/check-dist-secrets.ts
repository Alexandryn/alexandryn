import { requireDir, reportFindingsAndExit } from './checks/cli.ts'
import { findSecrets } from './checks/secretsGrep.ts'

const distDir = process.argv[2] ?? 'dist'
requireDir(distDir, 'check-dist-secrets')

reportFindingsAndExit(
  'check-dist-secrets',
  findSecrets(distDir),
  'secret-shaped strings found in the production bundle:',
)
