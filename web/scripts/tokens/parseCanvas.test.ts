import { describe, expect, it } from 'vitest'
import { parseRootCustomProperties, countPxValues, countEmValues } from './parseCanvas.ts'

describe('parseRootCustomProperties', () => {
  it('extracts every --name:value pair from the rootRef style attribute', () => {
    const html = `<div ref="{{ rootRef }}" style="--bg:#F6F5F2;--tx:#1A1917;--sh:0 1px 2px rgba(24,22,20,.05);height:100vh;display:flex">`

    expect(parseRootCustomProperties(html)).toEqual({
      '--bg': '#F6F5F2',
      '--tx': '#1A1917',
      '--sh': '0 1px 2px rgba(24,22,20,.05)',
    })
  })

  it('throws a clear error when no rootRef style attribute exists', () => {
    expect(() => parseRootCustomProperties('<div>no root here</div>')).toThrow(/rootRef/)
  })
})

describe('countPxValues', () => {
  it("counts occurrences of a CSS property's px values across the whole file", () => {
    const html = 'a{border-radius:8px}b{border-radius:8px}c{border-radius:2px}'

    expect(countPxValues(html, 'border-radius')).toEqual({ 8: 2, 2: 1 })
  })

  it('returns an empty map when the property never appears', () => {
    expect(countPxValues('a{color:red}', 'border-radius')).toEqual({})
  })

  it('matches decimal px values, not just integers', () => {
    // Real bug from code review: the design reference's font-size scale
    // is dominated by half-pixel values (12.5px is its single most-
    // frequent font-size, ahead of any integer) — an integer-only regex
    // silently dropped all of them.
    const html = 'a{font-size:12.5px}b{font-size:12.5px}c{font-size:12px}'

    expect(countPxValues(html, 'font-size')).toEqual({ 12.5: 2, 12: 1 })
  })
})

describe('countEmValues', () => {
  it('counts positive and negative decimal em values', () => {
    const html = 'a{letter-spacing:.1em}b{letter-spacing:-.025em}c{letter-spacing:.1em}'

    expect(countEmValues(html, 'letter-spacing')).toEqual({ '.1': 2, '-.025': 1 })
  })
})
