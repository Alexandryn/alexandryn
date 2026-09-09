import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findRadixVersionSplits } from './radixDedup.ts'

// audit 0016 #164: every @radix-ui/* package should resolve to one
// version so the bundle ships one copy of the shared internals.

let dir: string | undefined
afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function lockfile(packages: Record<string, { version?: string }>): string {
  dir = mkdtempSync(join(tmpdir(), 'radix-dedup-'))
  const path = join(dir, 'package-lock.json')
  writeFileSync(path, JSON.stringify({ packages }))
  return path
}

describe('findRadixVersionSplits', () => {
  it('returns nothing when every @radix-ui package is single-version', () => {
    const path = lockfile({
      'node_modules/@radix-ui/react-dialog': { version: '1.1.23' },
      'node_modules/@radix-ui/react-primitive': { version: '2.1.10' },
      'node_modules/foo/node_modules/@radix-ui/react-primitive': { version: '2.1.10' },
      'node_modules/lodash': { version: '4.17.21' },
    })
    expect(findRadixVersionSplits(path)).toEqual([])
  })

  it('flags a @radix-ui package present at two versions', () => {
    const path = lockfile({
      'node_modules/@radix-ui/react-primitive': { version: '2.1.10' },
      'node_modules/@radix-ui/react-slider/node_modules/@radix-ui/react-primitive': {
        version: '2.0.0',
      },
      'node_modules/@radix-ui/react-context': { version: '1.2.2' },
    })
    expect(findRadixVersionSplits(path)).toEqual([
      { name: '@radix-ui/react-primitive', versions: ['2.0.0', '2.1.10'] },
    ])
  })

  it('ignores non-radix duplicates', () => {
    const path = lockfile({
      'node_modules/esbuild': { version: '0.25.12' },
      'node_modules/vite/node_modules/esbuild': { version: '0.28.2' },
      'node_modules/@radix-ui/react-toast': { version: '1.2.23' },
    })
    expect(findRadixVersionSplits(path)).toEqual([])
  })
})
