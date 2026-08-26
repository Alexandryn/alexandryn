import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { walkScannableFiles } from './walkDist.ts'

// Real finding from code review: the secrets/MSW checks only ever read
// distDir/assets, but Vite copies web/public/* straight to dist's own
// root (confirmed empirically against a real build — favicon.svg,
// icons.svg land at dist/, not dist/assets/). MSW's own setup
// (`npx msw init public/`) puts the generated mockServiceWorker.js in
// public/, so it lands at dist/mockServiceWorker.js — exactly the file
// these checks exist to catch, previously invisible to them.

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

describe('walkScannableFiles', () => {
  it('finds text-shaped files at the dist root, not just under assets/', () => {
    dir = mkdtempSync(join(tmpdir(), 'walk-dist-'))
    mkdirSync(join(dir, 'assets'))
    writeFileSync(join(dir, 'assets', 'index-abc.js'), 'a')
    writeFileSync(join(dir, 'mockServiceWorker.js'), 'b') // public/ copied to dist root
    writeFileSync(join(dir, 'index.html'), 'c')

    const found = walkScannableFiles(dir).sort()

    expect(found).toEqual(
      [
        join(dir, 'assets', 'index-abc.js'),
        join(dir, 'index.html'),
        join(dir, 'mockServiceWorker.js'),
      ].sort(),
    )
  })

  it('recurses into arbitrarily nested directories', () => {
    dir = mkdtempSync(join(tmpdir(), 'walk-dist-'))
    mkdirSync(join(dir, 'a', 'b'), { recursive: true })
    writeFileSync(join(dir, 'a', 'b', 'deep.js'), 'x')

    expect(walkScannableFiles(dir)).toEqual([join(dir, 'a', 'b', 'deep.js')])
  })

  it('skips binary/non-scannable file types', () => {
    dir = mkdtempSync(join(tmpdir(), 'walk-dist-'))
    writeFileSync(join(dir, 'favicon.svg'), '<svg/>')
    writeFileSync(join(dir, 'photo.png'), Buffer.from([0, 1, 2]))

    expect(walkScannableFiles(dir)).toEqual([])
  })
})
