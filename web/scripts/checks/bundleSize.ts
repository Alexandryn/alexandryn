import { gzipSync } from 'node:zlib'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

// Total JS budget (all chunks combined).
export const BUNDLE_SIZE_BUDGET_BYTES = 250 * 1024

// Entry chunk gate:
// The entry chunk (index-*.js) is the initial payload — lazy routes
// must not silently collapse back into it. Vite names the entry chunk
// "index-<hash>.js". Budget is 150 KiB gzipped (15% headroom before triggering on regressions).
export const ENTRY_CHUNK_BUDGET_BYTES = 150 * 1024

/**
 * Identifies the Vite entry chunk by name (index-<hash>.js).
 * Returns null if no such file exists (Vite version/config changed).
 */
export function findEntryChunk(distDir: string): string | null {
  const assetsDir = join(distDir, 'assets')
  for (const name of readdirSync(assetsDir)) {
    if (name.startsWith('index-') && name.endsWith('.js')) return join(assetsDir, name)
  }
  return null
}

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
