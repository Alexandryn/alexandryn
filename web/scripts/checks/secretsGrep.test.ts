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
  const assets = join(dir, 'assets')
  mkdirSync(assets)
  for (const [name, content] of Object.entries(files)) {
    writeFileSync(join(assets, name), content)
  }
  return dir
}

describe('findSecrets', () => {
  it('finds nothing in an ordinary bundle', () => {
    const distDir = makeDist({ 'index-abc.js': 'function App(){return "hello world"}' })

    expect(findSecrets(distDir)).toEqual([])
  })

  it('flags an AWS access key ID', () => {
    const distDir = makeDist({ 'index-abc.js': 'const x = "AKIAIOSFODNN7EXAMPLE"' })

    const found = findSecrets(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/index-abc\.js$/)
  })

  it('flags a generic apiKey-shaped assignment', () => {
    const distDir = makeDist({ 'index-abc.js': 'apiKey: "sk_live_abcdefghijklmnopqrstuvwx"' })

    expect(findSecrets(distDir)).toHaveLength(1)
  })

  it('ignores non-.js assets', () => {
    const distDir = makeDist({ 'index-abc.css': 'AKIAIOSFODNN7EXAMPLE {color:red}' })

    expect(findSecrets(distDir)).toEqual([])
  })
})
