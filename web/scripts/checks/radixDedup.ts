import { readFileSync } from 'node:fs'

interface LockPackage {
  version?: string
}

interface Lockfile {
  packages?: Record<string, LockPackage>
}

/**
 * Returns the @radix-ui/* packages that resolve to more than one version
 * in the lockfile. Every Radix primitive shares a small set of internal
 * helpers (react-primitive, react-context, compose-refs …); a version
 * split means the browser bundle ships two copies of that shared code
 * (audit 0016 #164). One version per package is the healthy state; this
 * check keeps it that way.
 */
export function findRadixVersionSplits(
  lockfilePath: string,
): Array<{ name: string; versions: string[] }> {
  const lock = JSON.parse(readFileSync(lockfilePath, 'utf8')) as Lockfile
  const byName = new Map<string, Set<string>>()

  for (const [path, pkg] of Object.entries(lock.packages ?? {})) {
    if (!pkg.version) continue
    const idx = path.lastIndexOf('node_modules/')
    if (idx === -1) continue
    const name = path.slice(idx + 'node_modules/'.length)
    if (!name.startsWith('@radix-ui/')) continue
    let set = byName.get(name)
    if (!set) {
      set = new Set()
      byName.set(name, set)
    }
    set.add(pkg.version)
  }

  const splits: Array<{ name: string; versions: string[] }> = []
  for (const [name, versions] of byName) {
    if (versions.size > 1) {
      splits.push({ name, versions: [...versions].sort() })
    }
  }
  return splits.sort((a, b) => a.name.localeCompare(b.name))
}
