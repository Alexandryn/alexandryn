import { existsSync } from 'node:fs'
import { findMswReferences } from './checks/mswExclusion.ts'

const distDir = process.argv[2] ?? 'dist'

if (!existsSync(distDir)) {
  console.error(`check-dist-msw: ${distDir} does not exist — run "npm run build" first`)
  process.exit(1)
}

const findings = findMswReferences(distDir)

if (findings.length > 0) {
  console.error('check-dist-msw: MSW reference found in the production bundle:')
  for (const f of findings) {
    console.error(`  ${f.file}: matched ${f.pattern}`)
  }
  process.exit(1)
}

console.log('check-dist-msw: clean')
