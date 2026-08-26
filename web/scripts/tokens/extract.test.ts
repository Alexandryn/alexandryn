import { describe, expect, it } from 'vitest'
import { extractTokens } from './extract.ts'

function canvas(rootStyle: string, rest = ''): string {
  return `<div ref="{{ rootRef }}" style="${rootStyle}">${rest}</div>`
}

describe('extractTokens', () => {
  it('keeps a non-color custom property (a size) out of the colors list', () => {
    // Real bug: --cov (a px size) was being named through COLOR_NAMES and
    // emitted as `--color-cover-width: 112px`, a nonsensical Tailwind
    // color token.
    const tokens = extractTokens({
      electron: canvas('--bg:#F6F5F2;--cov:112px'),
    })

    expect(tokens.colors.find((c) => c.varName === '--cov')).toBeUndefined()
    expect(tokens.sizes).toEqual([
      { name: 'cover-width', varName: '--cov', value: '112px', canvases: ['electron'] },
    ])
  })

  it('names letter-spacing tokens without double-prefixing and flags the real most-frequent value', () => {
    // Real bug: names were generated as "tracking-1" and then the CSS
    // builder prefixed them again into "--tracking-tracking-1".
    const html = canvas(
      '--bg:#F6F5F2',
      'a{letter-spacing:.1em}'.repeat(10) +
        'b{letter-spacing:.08em}'.repeat(3) +
        'c{letter-spacing:-.02em}'.repeat(3),
    )
    const tokens = extractTokens({ electron: html })

    expect(tokens.letterSpacing.map((t) => t.name)).toEqual(['1', '2', '3'])
    const highestUsage = tokens.letterSpacing.find((t) => t.em === '.1')
    expect(highestUsage?.isDefault).toBe(true)
    expect(tokens.letterSpacing.filter((t) => t.isDefault)).toHaveLength(1)
  })
})
