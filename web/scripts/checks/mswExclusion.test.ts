import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findMswReferences } from './mswExclusion.ts'

// frontend-shell-and-routing.md FR-6 requires MSW for development/
// testing; this is the production-exclusion check that requirement
// depends on.

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function makeDist(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'msw-exclusion-'))
  for (const [name, content] of Object.entries(files)) {
    const full = join(dir, name)
    mkdirSync(join(full, '..'), { recursive: true })
    writeFileSync(full, content)
  }
  return dir
}

describe('findMswReferences', () => {
  it('finds nothing in an ordinary production bundle', () => {
    const distDir = makeDist({ 'assets/index-abc.js': 'function App(){return "hello world"}' })

    expect(findMswReferences(distDir)).toEqual([])
  })

  it('flags a reference to the generated mockServiceWorker file inside a JS chunk', () => {
    const distDir = makeDist({
      'assets/index-abc.js': 'navigator.serviceWorker.register("/mockServiceWorker.js")',
    })

    const found = findMswReferences(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/index-abc\.js$/)
  })

  it('flags the generated worker file itself, copied to the dist root from public/', () => {
    // Real finding from code review: Vite copies web/public/* straight
    // to dist's own root, not dist/assets/ — and MSW's own setup
    // (`npx msw init public/`) puts mockServiceWorker.js there. A check
    // scoped to assets/ alone would never see this file at all.
    const distDir = makeDist({
      'mockServiceWorker.js': '// Mock Service Worker (mockServiceWorker.js)',
      'assets/index-abc.js': 'function App(){}',
    })

    const found = findMswReferences(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/mockServiceWorker\.js$/)
  })

  it('does not false-positive on unrelated substrings', () => {
    const distDir = makeDist({
      'assets/index-abc.js': 'const msword = "not msw"; const worker = 1',
    })

    expect(findMswReferences(distDir)).toEqual([])
  })
})
