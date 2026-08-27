import { readdirSync, statSync } from 'node:fs'
import { extname, join } from 'node:path'

const SOURCE_EXTENSIONS = new Set(['.ts', '.tsx'])

/** Recursively lists every .ts/.tsx file under dir. */
export function walkSourceFiles(dir: string): string[] {
  const results: string[] = []
  const walk = (current: string): void => {
    for (const name of readdirSync(current)) {
      const full = join(current, name)
      if (statSync(full).isDirectory()) {
        walk(full)
      } else if (SOURCE_EXTENSIONS.has(extname(name))) {
        results.push(full)
      }
    }
  }
  walk(dir)
  return results
}
