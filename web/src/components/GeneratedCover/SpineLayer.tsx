import type { CSSProperties, HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import type { CoverSeed } from '../../lib/fnv1a'

export interface SpineLayerProps extends Omit<HTMLAttributes<HTMLDivElement>, 'aria-hidden'> {
  seed: CoverSeed
}

/** Pure — the same seed always produces the same spine color. Hue is procedural per-book, not a token color. */
export function SpineLayer({ seed, className, style, ...rest }: SpineLayerProps) {
  // Caller-supplied style first — the seed-derived backgroundColor comes
  // after so it can't be silently overridden (same reasoning as
  // TextureLayer's own merge order).
  const computedStyle: CSSProperties = { ...style, backgroundColor: `hsl(${seed.hue} 55% 45%)` }
  return (
    <div
      className={cx('absolute inset-y-0 left-0 w-xs', className)}
      style={computedStyle}
      {...rest}
      aria-hidden="true"
    />
  )
}
