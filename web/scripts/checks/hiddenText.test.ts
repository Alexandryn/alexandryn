import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findHandRolledHiddenText } from './hiddenText.ts'

// frontend-accessibility.md FR-3 / Acceptance criterion 4: every
// visually-hidden label uses <VisuallyHidden> (or Tailwind sr-only),
// never display:none (drops it from the a11y tree) or a hand-rolled
// clip-rect — proven by a grep-based check.

let dir: string | undefined
afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function tree(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'hidden-text-'))
  for (const [name, content] of Object.entries(files)) {
    const full = join(dir, name)
    mkdirSync(join(full, '..'), { recursive: true })
    writeFileSync(full, content)
  }
  return dir
}

describe('findHandRolledHiddenText', () => {
  it('flags display:none in a CSS-in-JS style object', () => {
    const found = findHandRolledHiddenText(
      tree({ 'a.tsx': "<span style={{ display: 'none' }}>Loading</span>" }),
    )
    expect(found).toHaveLength(1)
  })

  it('flags a hand-rolled clip-rect', () => {
    const found = findHandRolledHiddenText(
      tree({ 'b.tsx': '<span className="absolute w-px h-px overflow-hidden">x</span>' }),
    )
    expect(found).toHaveLength(1)
  })

  it('allows Tailwind sr-only', () => {
    const found = findHandRolledHiddenText(
      tree({ 'c.tsx': '<a className="sr-only focus:not-sr-only">Skip to content</a>' }),
    )
    expect(found).toEqual([])
  })

  it('allows a line with an explicit escape-hatch comment', () => {
    const found = findHandRolledHiddenText(
      tree({
        'd.tsx':
          "// a11y-hidden-text-ok: collapses a non-text layout region, no announcement intended\n<div style={{ display: 'none' }} />",
      }),
    )
    expect(found).toEqual([])
  })

  it('ignores test files', () => {
    const found = findHandRolledHiddenText(
      tree({ 'e.test.tsx': "expect(el).not.toHaveStyle('display: none')" }),
    )
    expect(found).toEqual([])
  })
})
