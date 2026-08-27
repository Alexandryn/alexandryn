import { describe, expect, it } from 'vitest'
import { extractTokens } from './extract.ts'

// Every real canvas carries the atTablet responsive-band prose that
// extractBreakpoint reads; include it here so extractTokens (which now
// also extracts the breakpoint) has it, without each case restating it.
const TABLET_BAND = '<p>768–1023px. The sidebar becomes a 60px icon rail.</p>'

function canvas(rootStyle: string, rest = ''): string {
  return `<div ref="{{ rootRef }}" style="${rootStyle}">${rest}${TABLET_BAND}</div>`
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

  it('throws loudly on an unrecognized custom property instead of silently dropping it', () => {
    // Real bug from code review: an unnamed property (not yet added to
    // COLOR_NAMES/SHADOW_NAMES/SIZE_NAMES) was just omitted from every
    // output category, with no error — the same silent-drop risk
    // mergeCanvasProperties already guards against for value
    // disagreements, previously missing here for unknown property names.
    expect(() => extractTokens({ electron: canvas('--bg:#F6F5F2;--newthing:#123456') })).toThrow(
      /--newthing/,
    )
  })

  it('extracts the FR-3 reflow breakpoint from the atTablet band prose', () => {
    const tokens = extractTokens({ electron: canvas('--bg:#F6F5F2') })
    expect(tokens.breakpoint).toEqual({ name: 'reflow', px: 768 })
  })
})
