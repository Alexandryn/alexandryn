import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findSecrets } from './secretsGrep.ts'

// frontend-tooling.md Security considerations: "grep for common
// secret-shaped patterns in web/dist output," a build-time CI step.

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function makeDist(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'secrets-grep-'))
  for (const [name, content] of Object.entries(files)) {
    const full = join(dir, name)
    mkdirSync(join(full, '..'), { recursive: true })
    writeFileSync(full, content)
  }
  return dir
}

describe('findSecrets', () => {
  it('finds nothing in an ordinary bundle', () => {
    const distDir = makeDist({ 'assets/index-abc.js': 'function App(){return "hello world"}' })

    expect(findSecrets(distDir)).toEqual([])
  })

  it('flags an AWS access key ID', () => {
    const distDir = makeDist({ 'assets/index-abc.js': 'const x = "AKIAIOSFODNN7EXAMPLE"' })

    const found = findSecrets(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/index-abc\.js$/)
  })

  it('flags a generic apiKey-shaped assignment', () => {
    const distDir = makeDist({
      'assets/index-abc.js': 'apiKey: "sk_live_abcdefghijklmnopqrstuvwx"',
    })

    expect(findSecrets(distDir)).toHaveLength(1)
  })

  it('flags a secret baked into index.html at the dist root, not just assets/', () => {
    // Real finding from code review: the previous version only ever
    // read distDir/assets, missing dist's own root entirely — where
    // index.html and anything Vite copies from public/ actually land.
    const distDir = makeDist({
      'index.html': '<script>window.__KEY__="AKIAIOSFODNN7EXAMPLE"</script>',
      'assets/index-abc.js': 'function App(){}',
    })

    const found = findSecrets(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/index\.html$/)
  })

  it('ignores non-scannable (binary/image) assets', () => {
    const distDir = makeDist({ 'favicon.svg': 'AKIAIOSFODNN7EXAMPLE' })

    expect(findSecrets(distDir)).toEqual([])
  })
})
