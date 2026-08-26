import { describe, expect, it } from 'vitest'
import { contrastRatio, meetsWcagAA } from './contrast.ts'

describe('contrastRatio', () => {
  it('is 21:1 for pure black on pure white (WCAG worked example)', () => {
    expect(contrastRatio('#000000', '#FFFFFF')).toBeCloseTo(21, 1)
  })

  it('is 1:1 for a color against itself', () => {
    expect(contrastRatio('#41608F', '#41608F')).toBeCloseTo(1, 5)
  })

  it('is symmetric regardless of argument order', () => {
    expect(contrastRatio('#1A1917', '#F6F5F2')).toBeCloseTo(contrastRatio('#F6F5F2', '#1A1917'), 5)
  })
})

describe('meetsWcagAA', () => {
  it('passes a real high-contrast pair from the design reference (text on background)', () => {
    expect(meetsWcagAA('#1A1917', '#F6F5F2')).toBe(true)
  })

  it('fails a deliberately low-contrast pair', () => {
    expect(meetsWcagAA('#F6F5F2', '#FBFAF9')).toBe(false)
  })
})
