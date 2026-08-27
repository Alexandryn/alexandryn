import { readdirSync, statSync } from 'node:fs'
import { extname, join } from 'node:path'

/** Recursively lists every file under dir whose extension is in extensions. */
export function walkFilesByExtension(dir: string, extensions: ReadonlySet<string>): string[] {
  const results: string[] = []
  const walk = (current: string): void => {
    for (const name of readdirSync(current)) {
      const full = join(current, name)
      if (statSync(full).isDirectory()) {
        walk(full)
      } else if (extensions.has(extname(name))) {
        results.push(full)
      }
    }
  }
  walk(dir)
  return results
}
