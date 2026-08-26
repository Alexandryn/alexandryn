import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findMswReferences } from './mswExclusion.ts'

// frontend-tooling.md Security considerations: MSW's bundle must not
// leak into a production build (frontend-shell-and-routing.md FR-6
// requires MSW for development/testing; this is the production-
// exclusion check that requirement depends on).

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function makeDist(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'msw-exclusion-'))
  const assets = join(dir, 'assets')
  mkdirSync(assets)
  for (const [name, content] of Object.entries(files)) {
    writeFileSync(join(assets, name), content)
  }
  return dir
}

describe('findMswReferences', () => {
  it('finds nothing in an ordinary production bundle', () => {
    const distDir = makeDist({ 'index-abc.js': 'function App(){return "hello world"}' })

    expect(findMswReferences(distDir)).toEqual([])
  })

  it('flags an import from the msw package', () => {
    const distDir = makeDist({ 'index-abc.js': 'import{setupWorker}from"msw/browser"' })

    const found = findMswReferences(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/index-abc\.js$/)
  })

  it('flags a reference to the generated mockServiceWorker file', () => {
    const distDir = makeDist({
      'index-abc.js': 'navigator.serviceWorker.register("/mockServiceWorker.js")',
    })

    expect(findMswReferences(distDir)).toHaveLength(1)
  })

  it('does not false-positive on unrelated substrings', () => {
    const distDir = makeDist({ 'index-abc.js': 'const msword = "not msw"; const worker = 1' })

    expect(findMswReferences(distDir)).toEqual([])
  })
})
