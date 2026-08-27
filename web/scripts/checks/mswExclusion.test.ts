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

  it('flags the generated worker file by name, even with a body that never self-references', () => {
    // Vite copies web/public/* straight to dist's own root, and
    // `npx msw init public/` puts mockServiceWorker.js there. Verified in
    // Tier 4: the real MSW 2.15 worker script contains no "mockServiceWorker"
    // string in its body, so this must be caught by filename, not content.
    const distDir = makeDist({
      'mockServiceWorker.js':
        '/*! Mock Service Worker. Do not register this file. */\nself.addEventListener("install", () => {})',
      'assets/index-abc.js': 'function App(){}',
    })

    const found = findMswReferences(distDir)

    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/mockServiceWorker\.js$/)
    expect(found[0]?.pattern).toContain('filename')
  })

  it('does not false-positive on unrelated substrings', () => {
    const distDir = makeDist({
      'assets/index-abc.js': 'const msword = "not msw"; const worker = 1',
    })

    expect(findMswReferences(distDir)).toEqual([])
  })
})
