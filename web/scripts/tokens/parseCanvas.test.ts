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
})

describe('countEmValues', () => {
  it('counts positive and negative decimal em values', () => {
    const html = 'a{letter-spacing:.1em}b{letter-spacing:-.025em}c{letter-spacing:.1em}'

    expect(countEmValues(html, 'letter-spacing')).toEqual({ '.1': 2, '-.025': 1 })
  })
})
