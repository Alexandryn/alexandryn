const FNV_OFFSET_BASIS = 0x811c9dc5
const FNV_PRIME = 0x01000193

/** FNV-1a, 32-bit. Deterministic, non-cryptographic — chosen for being boring and dependency-free. */
export function fnv1a(input: string): number {
  let hash = FNV_OFFSET_BASIS
  for (let i = 0; i < input.length; i++) {
    hash ^= input.charCodeAt(i)
    hash = Math.imul(hash, FNV_PRIME)
  }
  return hash >>> 0
}

export type PatternVariant = 'flat' | 'diagonal-stripe' | 'dot-grid'

const PATTERN_VARIANTS: readonly PatternVariant[] = ['flat', 'diagonal-stripe', 'dot-grid']

export interface CoverSeed {
  hue: number
  pattern: PatternVariant
}

/**
 * Derives a cover's texture/spine seed from a book identifier (Work.ID or
 * Edition.ID). Hue and pattern are hashed from distinct salted strings so
 * they don't correlate with each other for the same identifier.
 */
export function deriveSeed(identifier: string): CoverSeed {
  const hueHash = fnv1a(identifier)
  const patternHash = fnv1a(`${identifier}:pattern`)
  return {
    hue: hueHash % 360,
    pattern: PATTERN_VARIANTS[patternHash % PATTERN_VARIANTS.length]!,
  }
}
