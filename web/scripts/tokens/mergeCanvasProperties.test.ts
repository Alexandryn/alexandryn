import { describe, expect, it } from 'vitest'
import { mergeCanvasProperties } from './mergeCanvasProperties.ts'

describe('mergeCanvasProperties', () => {
  it('unions properties across canvases, recording which canvases declared each', () => {
    const merged = mergeCanvasProperties({
      electron: { '--bg': '#F6F5F2', '--cov': '112px' },
      web: { '--bg': '#F6F5F2' },
    })

    expect(merged).toEqual({
      '--bg': { value: '#F6F5F2', canvases: ['electron', 'web'] },
      '--cov': { value: '112px', canvases: ['electron'] },
    })
  })

  it('throws if two canvases declare the same property with different values', () => {
    expect(() =>
      mergeCanvasProperties({
        electron: { '--bg': '#F6F5F2' },
        web: { '--bg': '#000000' },
      }),
    ).toThrow(/--bg.*electron.*web|--bg.*disagree/i)
  })
})
