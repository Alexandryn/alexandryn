import { describe, expect, it } from 'vitest'
import { extractBreakpoint } from './extractBreakpoint.ts'

describe('extractBreakpoint', () => {
  it('reads the low end of the atTablet responsive band as the reflow breakpoint', () => {
    const admin =
      '<div><h1>Tablet</h1><div>768–1023px. The sidebar becomes a 60px icon rail, ' +
      'the book page goes single-column, and filters move into a sheet.</div></div>'

    expect(extractBreakpoint({ admin, other: '<div>no band here</div>' })).toEqual({
      name: 'reflow',
      px: 768,
    })
  })

  it('accepts a plain hyphen as well as an en dash', () => {
    const admin = '<div>640-1023px. The sidebar becomes a rail</div>'
    expect(extractBreakpoint({ admin }).px).toBe(640)
  })

  it('throws loudly when the band prose is absent (the design reference changed)', () => {
    expect(() => extractBreakpoint({ admin: '<div>nothing responsive here</div>' })).toThrow(
      /design reference/i,
    )
  })
})
