import { gzipSync } from 'node:zlib'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

// frontend-tooling.md FR-4: 250 KiB gzipped, the initial JS payload
// specifically — CSS/HTML/images aren't part of this budget.
export const BUNDLE_SIZE_BUDGET_BYTES = 250 * 1024

/** Sums the gzipped size of every .js file under distDir/assets. */
export function measureJsGzipBytes(distDir: string): number {
  const assetsDir = join(distDir, 'assets')
  let total = 0
  for (const name of readdirSync(assetsDir)) {
    if (!name.endsWith('.js')) continue
    const contents = readFileSync(join(assetsDir, name))
    total += gzipSync(contents).length
  }
  return total
}
