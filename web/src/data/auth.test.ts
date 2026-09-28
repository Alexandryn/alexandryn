import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import {
  clearSession,
  getActiveLibraryId,
  parseJwtLibraries,
  setActiveLibraryId,
  setAccessToken,
} from './auth'

describe('auth token and active library management', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    localStorage.clear()
  })

  // Helper to construct an unsigned JWT for testing claims
  function makeTestJwt(payload: Record<string, unknown>): string {
    const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
    const body = btoa(JSON.stringify(payload))
    return `${header}.${body}.signature`
  }

  it('extracts library claims from valid JWT payload', () => {
    const token = makeTestJwt({
      sub: 'u-123',
      libraries: ['00000000-0000-0000-0000-000000000001', 'lib-custom'],
    })
    expect(parseJwtLibraries(token)).toEqual([
      '00000000-0000-0000-0000-000000000001',
      'lib-custom',
    ])
  })

  it('handles tokens without libraries or malformed tokens safely', () => {
    expect(parseJwtLibraries('')).toEqual([])
    expect(parseJwtLibraries('invalid.token')).toEqual([])
    const noLibs = makeTestJwt({ sub: 'u-123' })
    expect(parseJwtLibraries(noLibs)).toEqual([])
  })

  it('automatically sets active library on setAccessToken when none is currently set', () => {
    expect(getActiveLibraryId()).toBeNull()

    const token = makeTestJwt({
      sub: 'u-123',
      libraries: ['lib-prime'],
    })
    setAccessToken(token)

    expect(getActiveLibraryId()).toBe('lib-prime')
  })

  it('preserves existing active library if already selected by the user', () => {
    setActiveLibraryId('existing-lib')
    expect(getActiveLibraryId()).toBe('existing-lib')

    const token = makeTestJwt({
      sub: 'u-123',
      libraries: ['other-lib'],
    })
    setAccessToken(token)

    expect(getActiveLibraryId()).toBe('existing-lib')
  })

  it('clears active library when clearSession is invoked', () => {
    setActiveLibraryId('lib-to-clear')
    expect(getActiveLibraryId()).toBe('lib-to-clear')

    clearSession()

    expect(getActiveLibraryId()).toBeNull()
  })
})
