import { describe, expect, it } from 'vitest'
import { defaultOpenLibraryUserAgent } from './serverConfigDefaults'

describe('defaultOpenLibraryUserAgent', () => {
  it('names the app, the packaged version, and a URL identifying the project', () => {
    const ua = defaultOpenLibraryUserAgent('1.0.2')
    expect(ua).toContain('Alexandryn-Desktop/1.0.2')
    expect(ua).toContain('https://github.com/Alexandryn/alexandryn')
  })

  it('has no whitespace or control characters Open Library could choke on', () => {
    const ua = defaultOpenLibraryUserAgent('1.0.2')
    expect(ua).not.toMatch(/[\r\n\t]/)
  })

  it('changes with the version, so a report against Open Library is traceable', () => {
    expect(defaultOpenLibraryUserAgent('1.2.3')).not.toBe(defaultOpenLibraryUserAgent('4.5.6'))
  })
})
