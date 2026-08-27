import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { walkFilesByExtension } from './walkFiles.ts'

// Shared by walkDist.ts (check-dist-secrets/check-dist-msw) and walkSrc.ts
// (check-token-styling) — each just names its own extension set, rather
// than repeating the same recursive-directory-walk logic.

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

describe('walkFilesByExtension', () => {
  it('lists only files matching the given extensions', () => {
    dir = mkdtempSync(join(tmpdir(), 'walk-files-'))
    writeFileSync(join(dir, 'a.ts'), 'x')
    writeFileSync(join(dir, 'b.png'), 'x')

    expect(walkFilesByExtension(dir, new Set(['.ts']))).toEqual([join(dir, 'a.ts')])
  })

  it('recurses into nested directories', () => {
    dir = mkdtempSync(join(tmpdir(), 'walk-files-'))
    mkdirSync(join(dir, 'a', 'b'), { recursive: true })
    writeFileSync(join(dir, 'a', 'b', 'deep.ts'), 'x')

    expect(walkFilesByExtension(dir, new Set(['.ts']))).toEqual([join(dir, 'a', 'b', 'deep.ts')])
  })
})
