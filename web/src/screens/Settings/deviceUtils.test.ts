import { describe, expect, it } from 'vitest'
import { buildKindLine, formatRelativeTime, isActiveDot } from './deviceUtils'

describe('deviceUtils unit tests (Phase 14)', () => {
  const pinnedNow = new Date('2026-09-06T12:00:00Z')

  describe('formatRelativeTime', () => {
    it('returns "Active now" when less than 5 minutes ago', () => {
      const fourMinAgo = new Date(pinnedNow.getTime() - 4 * 60 * 1000).toISOString()
      expect(formatRelativeTime(fourMinAgo, pinnedNow)).toBe('Active now')
    })

    it('returns "Active 37 minutes ago" when 37 minutes ago', () => {
      const thirtySevenMinAgo = new Date(pinnedNow.getTime() - 37 * 60 * 1000).toISOString()
      expect(formatRelativeTime(thirtySevenMinAgo, pinnedNow)).toBe('Active 37 minutes ago')
    })

    it('returns "Active 3 hours ago" when 3 hours ago', () => {
      const threeHoursAgo = new Date(pinnedNow.getTime() - 3 * 3600 * 1000).toISOString()
      expect(formatRelativeTime(threeHoursAgo, pinnedNow)).toBe('Active 3 hours ago')
    })

    it('returns a date string (not "Active N hours ago") when 25 hours ago', () => {
      const twentyFiveHoursAgo = new Date(pinnedNow.getTime() - 25 * 3600 * 1000).toISOString()
      const formatted = formatRelativeTime(twentyFiveHoursAgo, pinnedNow)
      expect(formatted).not.toContain('Active')
      expect(formatted).not.toContain('ago')
      expect(formatted).toMatch(/Sep 5, 2026/)
    })

    it('returns "Never synced" for null or undefined', () => {
      expect(formatRelativeTime(null, pinnedNow)).toBe('Never synced')
      expect(formatRelativeTime(undefined, pinnedNow)).toBe('Never synced')
    })

    it('supports custom prefix (e.g. Synced)', () => {
      const fourMinAgo = new Date(pinnedNow.getTime() - 4 * 60 * 1000).toISOString()
      expect(formatRelativeTime(fourMinAgo, pinnedNow, 'Synced')).toBe('Synced now')

      const twentyMinAgo = new Date(pinnedNow.getTime() - 20 * 60 * 1000).toISOString()
      expect(formatRelativeTime(twentyMinAgo, pinnedNow, 'Synced')).toBe('Synced 20 minutes ago')
    })
  })

  describe('isActiveDot', () => {
    it('returns true when lastSeenAt is within 5 minutes', () => {
      const fourMinFiftyNineSecAgo = new Date(pinnedNow.getTime() - (5 * 60 * 1000 - 1000)).toISOString()
      expect(isActiveDot(fourMinFiftyNineSecAgo, pinnedNow)).toBe(true)
    })

    it('returns false at boundary (exactly 5 minutes ago) and beyond', () => {
      const exactlyFiveMinAgo = new Date(pinnedNow.getTime() - 5 * 60 * 1000).toISOString()
      expect(isActiveDot(exactlyFiveMinAgo, pinnedNow)).toBe(false)

      const sixMinAgo = new Date(pinnedNow.getTime() - 6 * 60 * 1000).toISOString()
      expect(isActiveDot(sixMinAgo, pinnedNow)).toBe(false)
    })

    it('returns false for null/undefined or invalid date', () => {
      expect(isActiveDot(null, pinnedNow)).toBe(false)
      expect(isActiveDot(undefined, pinnedNow)).toBe(false)
      expect(isActiveDot('invalid-date', pinnedNow)).toBe(false)
    })
  })

  describe('buildKindLine', () => {
    it('formats phone with pairing_code correctly', () => {
      expect(buildKindLine('phone', 'pairing_code')).toBe('Phone · paired by code')
    })

    it('formats desktop with password_login correctly', () => {
      expect(buildKindLine('desktop', 'password_login')).toBe('Desktop · direct login')
    })

    it('formats tablet with pairing_code correctly', () => {
      expect(buildKindLine('tablet', 'pairing_code')).toBe('Tablet · paired by code')
    })
  })
})
