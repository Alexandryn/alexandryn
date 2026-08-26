import { existsSync } from 'node:fs'
import { BUNDLE_SIZE_BUDGET_BYTES, measureJsGzipBytes } from './checks/bundleSize.ts'

const distDir = process.argv[2] ?? 'dist'

if (!existsSync(distDir)) {
  console.error(`check-bundle-size: ${distDir} does not exist — run "npm run build" first`)
  process.exit(1)
}

const bytes = measureJsGzipBytes(distDir)
const budgetKiB = (BUNDLE_SIZE_BUDGET_BYTES / 1024).toFixed(0)
const actualKiB = (bytes / 1024).toFixed(1)

if (bytes > BUNDLE_SIZE_BUDGET_BYTES) {
  console.error(
    `check-bundle-size: ${actualKiB} KiB gzipped exceeds the ${budgetKiB} KiB budget (frontend-tooling.md FR-4)`,
  )
  process.exit(1)
}

console.log(`check-bundle-size: ${actualKiB} KiB gzipped, within the ${budgetKiB} KiB budget`)
