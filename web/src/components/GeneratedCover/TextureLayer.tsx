import type { CSSProperties, HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import type { CoverSeed } from '../../lib/fnv1a'

export interface TextureLayerProps extends Omit<HTMLAttributes<HTMLDivElement>, 'aria-hidden'> {
  seed: CoverSeed
}

function backgroundImageFor(seed: CoverSeed): string | undefined {
  const base = `hsl(${seed.hue} 38% 88%)`
  const accent = `hsl(${seed.hue} 38% 82%)`
  const unit = 'var(--spacing-xs)'
  switch (seed.pattern) {
    case 'flat':
      return undefined
    case 'diagonal-stripe':
      return `repeating-linear-gradient(45deg, ${accent}, ${accent} ${unit}, ${base} ${unit}, ${base} calc(${unit} * 2))`
    case 'dot-grid':
      return `radial-gradient(${accent} 15%, ${base} 16%)`
  }
}

/** Pure — the same seed always produces the same texture (FR-1/FR-2). Hue is procedural per-book, not a token color (D2). */
export function TextureLayer({ seed, className, style, ...rest }: TextureLayerProps) {
  const backgroundImage = backgroundImageFor(seed)
  // Caller-supplied style spreads first — the seed-derived background
  // properties come after so a caller can add unrelated style (e.g.
  // transform) without being able to silently override the values FR-2's
  // determinism guarantee depends on.
  const computedStyle: CSSProperties = {
    ...style,
    backgroundColor: `hsl(${seed.hue} 38% 88%)`,
    backgroundImage,
    backgroundSize:
      seed.pattern === 'dot-grid'
        ? 'calc(var(--spacing-sm) * 2) calc(var(--spacing-sm) * 2)'
        : undefined,
  }
  return (
    <div
      className={cx('size-full', className)}
      style={computedStyle}
      {...rest}
      aria-hidden="true"
    />
  )
}
