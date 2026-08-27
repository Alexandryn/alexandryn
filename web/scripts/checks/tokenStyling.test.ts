import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterEach, describe, expect, it } from 'vitest'
import { findRawStyleValues } from './tokenStyling.ts'

// frontend-component-primitives.md FR-4/Acceptance criteria: "no primitive's
// source contains a raw hex/px value outside the token set," proven by a
// grep-based check as an interim measure until a lint rule exists.

let dir: string | undefined

afterEach(() => {
  if (dir) rmSync(dir, { recursive: true, force: true })
  dir = undefined
})

function makeComponents(files: Record<string, string>): string {
  dir = mkdtempSync(join(tmpdir(), 'token-styling-'))
  for (const [name, content] of Object.entries(files)) {
    const full = join(dir, name)
    mkdirSync(join(full, '..'), { recursive: true })
    writeFileSync(full, content)
  }
  return dir
}

describe('findRawStyleValues', () => {
  it('finds nothing in token-only-styled source', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button className="bg-accent text-accent-text rounded-md px-lg" /> }`,
    })

    expect(findRawStyleValues(componentsDir)).toEqual([])
  })

  it('flags a raw hex color', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button style={{ color: '#1a1917' }} /> }`,
    })

    const found = findRawStyleValues(componentsDir)
    expect(found).toHaveLength(1)
    expect(found[0]?.file).toMatch(/Button\.tsx$/)
  })

  it('flags a Tailwind arbitrary pixel value', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button className="w-[16px]" /> }`,
    })

    expect(findRawStyleValues(componentsDir)).toHaveLength(1)
  })

  it('flags a raw pixel value in an inline style', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button style={{ width: '16px' }} /> }`,
    })

    expect(findRawStyleValues(componentsDir)).toHaveLength(1)
  })

  it('flags a CSS4 alpha-hex color (4 or 8 digits)', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button style={{ color: '#1a1917ff' }} /> }`,
    })

    expect(findRawStyleValues(componentsDir)).toHaveLength(1)
  })

  it('flags a bare-number pixel value in an inline style (React appends "px" itself)', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button style={{ width: 16 }} /> }`,
    })

    expect(findRawStyleValues(componentsDir)).toHaveLength(1)
  })

  it('does not flag prose that happens to read like a style property (no object-literal shape)', () => {
    const componentsDir = makeComponents({
      'VisuallyHidden/VisuallyHidden.test.tsx': `it('is clipped, never display:none or width:0 alone', () => {})`,
    })

    expect(findRawStyleValues(componentsDir)).toEqual([])
  })

  it('does not flag a unitless numeric style value', () => {
    const componentsDir = makeComponents({
      'Button/Button.tsx': `export function Button() { return <button style={{ lineHeight: 1.5, opacity: 0.5 }} /> }`,
    })

    expect(findRawStyleValues(componentsDir)).toEqual([])
  })

  it('ignores test and story files that only reference token classes', () => {
    const componentsDir = makeComponents({
      'Button/Button.stories.tsx': `export const Default = { args: { className: 'text-4xl' } }`,
    })

    expect(findRawStyleValues(componentsDir)).toEqual([])
  })
})
