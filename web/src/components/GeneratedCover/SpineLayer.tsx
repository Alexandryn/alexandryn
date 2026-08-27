import type { CSSProperties, HTMLAttributes } from 'react'
import { cx } from '../../lib/cx'
import type { CoverSeed } from '../../lib/fnv1a'

export interface SpineLayerProps extends HTMLAttributes<HTMLDivElement> {
  seed: CoverSeed
}

/** Pure — the same seed always produces the same spine color (FR-1/FR-2). Hue is procedural per-book, not a token color (D2). */
export function SpineLayer({ seed, className, style, ...rest }: SpineLayerProps) {
  const computedStyle: CSSProperties = { backgroundColor: `hsl(${seed.hue} 55% 45%)`, ...style }
  return (
    <div
      aria-hidden="true"
      className={cx('absolute inset-y-0 left-0 w-xs', className)}
      style={computedStyle}
      {...rest}
    />
  )
}
