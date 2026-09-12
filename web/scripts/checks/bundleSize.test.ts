import { gzipSync } from 'node:zlib'
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { BUNDLE_SIZE_BUDGET_BYTES, measureJsGzipBytes } from './bundleSize.ts'

// 250 KiB gzipped budget for the initial JS payload.
// Only .js assets count — CSS/HTML/images aren't part of this budget.

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function makeDist(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'bundle-size-'))
  const assets = join(dir, 'assets')
  mkdirSync(assets)
  for (const [name, content] of Object.entries(files)) {
    writeFileSync(join(assets, name), content)
  }
  return dir
}

describe('measureJsGzipBytes', () => {
  it('sums the gzipped size of every .js asset', () => {
    const a = 'x'.repeat(1000)
    const b = 'y'.repeat(2000)
    const distDir = makeDist({ 'index-abc.js': a, 'vendor-def.js': b, 'index-abc.css': 'body{}' })

    const bytes = measureJsGzipBytes(distDir)

    expect(bytes).toBe(gzipSync(Buffer.from(a)).length + gzipSync(Buffer.from(b)).length)
  })

  it('ignores non-.js assets entirely', () => {
    const distDir = makeDist({ 'index-abc.css': 'z'.repeat(5000), 'favicon-abc.svg': '<svg/>' })

    expect(measureJsGzipBytes(distDir)).toBe(0)
  })
})

describe('BUNDLE_SIZE_BUDGET_BYTES', () => {
  it('is 250 KiB', () => {
    expect(BUNDLE_SIZE_BUDGET_BYTES).toBe(250 * 1024)
  })
})
