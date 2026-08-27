import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findPositiveTabindex } from './positiveTabindex.ts'

// frontend-accessibility.md FR-2 / Acceptance criterion 3: no positive
// tabindex anywhere, proven by a grep-based check.

let dir: string | undefined
afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function tree(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'tabindex-'))
  for (const [name, content] of Object.entries(files)) {
    const full = join(dir, name)
    mkdirSync(join(full, '..'), { recursive: true })
    writeFileSync(full, content)
  }
  return dir
}

describe('findPositiveTabindex', () => {
  it('flags a positive tabIndex in JSX', () => {
    const found = findPositiveTabindex(tree({ 'a.tsx': '<div tabIndex={2}>x</div>' }))
    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/a\.tsx$/)
  })

  it('flags a positive tabindex in an HTML attribute string', () => {
    const found = findPositiveTabindex(tree({ 'b.html': '<span tabindex="1">x</span>' }))
    expect(found).toHaveLength(1)
  })

  it('allows tabIndex={-1} and tabIndex={0}', () => {
    const found = findPositiveTabindex(
      tree({ 'c.tsx': '<h1 tabIndex={-1}/>\n<main tabIndex={0}/>' }),
    )
    expect(found).toEqual([])
  })

  it('does not match unrelated text', () => {
    const found = findPositiveTabindex(
      tree({ 'd.tsx': '// tabIndex ordering: never use a value above 0\nconst tabIndex1 = 5' }),
    )
    expect(found).toEqual([])
  })
})
