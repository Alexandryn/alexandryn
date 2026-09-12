import { describe, expect, it, vi } from 'vitest'

import type { EpubSection } from '../../vendor/foliate/epub'
import { progressRatio, restoreScroll, sectionScrollFraction, selectionCfis } from './position'

const section = { cfi: '/6/4' } as EpubSection

describe('sectionScrollFraction', () => {
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

describe('selectionCfis', () => {
  const docWith = (html: string) =>
    new DOMParser().parseFromString(`<html><body>${html}</body></html>`, 'text/html')

  it('derives distinct start and end CFIs from a real text selection', () => {
    const doc = docWith('<p>Hello brave new world</p>')
    const text = doc.querySelector('p')!.firstChild as Text
    const range = doc.createRange()
    range.setStart(text, 6) // "brave..."
    range.setEnd(text, 20) // "...world"

    const { start, end } = selectionCfis(section, doc, range)

    expect(start).not.toBe(end)
    expect(start).toMatch(/^epubcfi\(\/6\/4!/)
    expect(end).toMatch(/^epubcfi\(\/6\/4!/)
  })

  it('spans element boundaries', () => {
    const doc = docWith('<p>first para</p><p>second para</p>')
    const [p1, p2] = Array.from(doc.querySelectorAll('p'))
    const range = doc.createRange()
    range.setStart(p1!.firstChild!, 0)
    range.setEnd(p2!.firstChild!, 6)

    const { start, end } = selectionCfis(section, doc, range)
    expect(start).not.toBe(end)
  })

  it('falls back to a single position when the range cannot be read', () => {
    const doc = docWith('<p>text</p>')
    const bad = { startContainer: null } as unknown as Range
    const { start, end } = selectionCfis(section, doc, bad)
    expect(start).toBe(end)
  })
})

describe('restoreScroll', () => {
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
