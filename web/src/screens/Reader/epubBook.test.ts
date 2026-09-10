import { describe, expect, it } from 'vitest'
import { contentUrl, flattenToc, sectionIndexForHref } from './epubBook'
import type { EpubSection, EpubTocItem } from '../../vendor/foliate/epub'

describe('epubBook', () => {
  describe('contentUrl', () => {
    it('encodes paths correctly without leading slash duplication', () => {
      expect(contentUrl('ed-1', 'OEBPS/ch 1.xhtml')).toBe(
        '/api/v1/library/editions/ed-1/reader/content/OEBPS/ch%201.xhtml',
      )
      expect(contentUrl('ed-1', '/OEBPS/ch1.xhtml')).toBe(
        '/api/v1/library/editions/ed-1/reader/content/OEBPS/ch1.xhtml',
      )
    })
  })

  describe('flattenToc', () => {
    it('flattens nested TOC trees with correct depth', () => {
      const items: EpubTocItem[] = [
        {
          label: 'Part 1',
          href: 'part1.xhtml',
          subitems: [
            { label: 'Chapter 1', href: 'ch1.xhtml' },
            { label: 'Chapter 2', href: 'ch2.xhtml' },
          ],
        },
        { label: 'Part 2', href: 'part2.xhtml' },
      ]

      const flat = flattenToc(items)
      expect(flat).toEqual([
        { label: 'Part 1', href: 'part1.xhtml', depth: 0 },
        { label: 'Chapter 1', href: 'ch1.xhtml', depth: 1 },
        { label: 'Chapter 2', href: 'ch2.xhtml', depth: 1 },
        { label: 'Part 2', href: 'part2.xhtml', depth: 0 },
      ])
    })
  })

  describe('sectionIndexForHref (audit 0016 #237)', () => {
    const mockSections: EpubSection[] = [
      {
        id: 'OEBPS/cover.xhtml',
        linear: 'yes',
        load: () => Promise.resolve(),
      } as unknown as EpubSection,
      {
        id: 'OEBPS/text/chapter1.xhtml',
        linear: 'yes',
        load: () => Promise.resolve(),
      } as unknown as EpubSection,
      {
        id: 'OEBPS/text/chapter2.xhtml',
        linear: 'yes',
        load: () => Promise.resolve(),
      } as unknown as EpubSection,
      {
        id: 'OEBPS/text/appendix_chapter2.xhtml',
        linear: 'yes',
        load: () => Promise.resolve(),
      } as unknown as EpubSection,
    ]

    it('finds exact match and ignores fragments', () => {
      expect(sectionIndexForHref(mockSections, 'OEBPS/cover.xhtml')).toBe(0)
      expect(sectionIndexForHref(mockSections, 'OEBPS/cover.xhtml#section-1')).toBe(0)
    })

    it('matches path when TOC has relative filename', () => {
      expect(sectionIndexForHref(mockSections, 'chapter1.xhtml')).toBe(1)
      expect(sectionIndexForHref(mockSections, 'text/chapter2.xhtml#heading')).toBe(2)
    })

    it('does not loosely match partial filename substrings (audit 0016 #237)', () => {
      // chapter2.xhtml must NOT match appendix_chapter2.xhtml
      expect(sectionIndexForHref(mockSections, 'chapter2.xhtml')).toBe(2)
      expect(sectionIndexForHref(mockSections, 'appendix_chapter2.xhtml')).toBe(3)
    })

    it('returns -1 for unresolvable or missing entries', () => {
      expect(sectionIndexForHref(mockSections, 'missing.xhtml')).toBe(-1)
      expect(sectionIndexForHref(mockSections, '')).toBe(-1)
    })
  })
})
