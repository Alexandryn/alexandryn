import { describe, expect, it, vi } from 'vitest'

import { progressRatio, restoreScroll, sectionScrollFraction } from './position'

describe('sectionScrollFraction (audit 0016 #145)', () => {
  it('returns the offset within the active section, not the whole book', () => {
    // 10 sections, book-wide 35% = section 3, halfway down it.
    expect(sectionScrollFraction(0.35, 3, 10)).toBeCloseTo(0.5)
    expect(sectionScrollFraction(0.3, 3, 10)).toBeCloseTo(0)
    expect(sectionScrollFraction(0.39, 3, 10)).toBeCloseTo(0.9)
  })

  it('round-trips with progressRatio', () => {
    expect(progressRatio(3, 10, sectionScrollFraction(0.35, 3, 10))).toBeCloseTo(0.35)
  })

  it('clamps to [0, 1] when the section index and percentage disagree', () => {
    // CFI resolved a later section than the stale percentage implies.
    expect(sectionScrollFraction(0.2, 3, 10)).toBe(0)
    // percentage runs past the section end (e.g. index came from CFI).
    expect(sectionScrollFraction(0.8, 3, 10)).toBe(1)
  })

  it('is zero for an empty or unknown book', () => {
    expect(sectionScrollFraction(0.5, 0, 0)).toBe(0)
    expect(sectionScrollFraction(0, 0, 5)).toBe(0)
  })
})

describe('restoreScroll (audit 0016 #145)', () => {
  const fakeWin = (scrollHeight: number, clientHeight: number) => {
    const scrollTo = vi.fn()
    return {
      scrollTo,
      document: { scrollingElement: { scrollHeight, clientHeight }, documentElement: {} },
    } as unknown as Parameters<typeof restoreScroll>[0] & { scrollTo: ReturnType<typeof vi.fn> }
  }

  it('scrolls to the saved fraction of the scrollable height', () => {
    const win = fakeWin(1000, 200)
    restoreScroll(win, 0.5)
    expect(win.scrollTo).toHaveBeenCalledWith(0, 400) // (1000 - 200) * 0.5
  })

  it('scrolls to the top for a zero or negative fraction', () => {
    const win = fakeWin(1000, 200)
    restoreScroll(win, 0)
    expect(win.scrollTo).toHaveBeenCalledWith(0, 0)
  })

  it('scrolls to the top when the document is not scrollable', () => {
    const win = fakeWin(200, 200)
    restoreScroll(win, 0.5)
    expect(win.scrollTo).toHaveBeenCalledWith(0, 0)
  })
})
