import { describe, expect, it } from 'vitest'
import { coverImageSrc, isAllowedImageDataUri } from './imageDataUri'

describe('isAllowedImageDataUri', () => {
  it('accepts the approved raster image types', () => {
    expect(isAllowedImageDataUri('data:image/jpeg;base64,/9j/xxx')).toBe(true)
    expect(isAllowedImageDataUri('data:image/png;base64,iVBOR')).toBe(true)
    expect(isAllowedImageDataUri('data:image/webp;base64,UklGR')).toBe(true)
    expect(isAllowedImageDataUri('DATA:IMAGE/JPEG,rawdata')).toBe(true)
  })

  it('rejects svg, html and other types', () => {
    expect(isAllowedImageDataUri('data:image/svg+xml,<svg onload=alert(1)>')).toBe(false)
    expect(isAllowedImageDataUri('data:text/html,<script>alert(1)</script>')).toBe(false)
    expect(isAllowedImageDataUri('data:application/octet-stream;base64,AAAA')).toBe(false)
    expect(isAllowedImageDataUri('javascript:alert(1)')).toBe(false)
    expect(isAllowedImageDataUri('data:,plain')).toBe(false)
  })
})

describe('coverImageSrc', () => {
  it('wraps raw base64 as JPEG', () => {
    expect(coverImageSrc('/9j/rawbytes')).toBe('data:image/jpeg;base64,/9j/rawbytes')
  })

  it('passes an allowed data URI through unchanged', () => {
    expect(coverImageSrc('data:image/png;base64,iVBOR')).toBe('data:image/png;base64,iVBOR')
  })

  it('returns null for a disallowed data URI so the caller falls back', () => {
    expect(coverImageSrc('data:image/svg+xml,<svg/>')).toBeNull()
    expect(coverImageSrc('data:text/html,x')).toBeNull()
  })

  it('returns null for no input', () => {
    expect(coverImageSrc(undefined)).toBeNull()
    expect(coverImageSrc('')).toBeNull()
    expect(coverImageSrc('   ')).toBeNull()
  })

  it('trims leading whitespace before deciding how to wrap', () => {
    expect(coverImageSrc('  data:image/png;base64,iVBOR')).toBe('data:image/png;base64,iVBOR')
    expect(coverImageSrc('\n/9j/rawbytes')).toBe('data:image/jpeg;base64,/9j/rawbytes')
  })

  it('drops an oversized value for the generated fallback', () => {
    expect(coverImageSrc('A'.repeat(1_500_001))).toBeNull()
    expect(coverImageSrc(`data:image/png;base64,${'A'.repeat(1_500_000)}`)).toBeNull()
  })
})
