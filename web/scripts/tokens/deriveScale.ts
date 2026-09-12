export interface ScaleToken {
  name: string
  px: number
  count: number
}

// Centered on "md" — enough labels either side to cover any realistic
// scale size derived from real design-reference frequency data.
const LABELS = [
  '5xs',
  '4xs',
  '3xs',
  '2xs',
  'xs',
  'sm',
  'md',
  'lg',
  'xl',
  '2xl',
  '3xl',
  '4xl',
  '5xl',
]

/**
 * Picks `count` consecutive labels from `labels`, centered on
 * `centerLabel` as evenly as possible — the same "grow outward from a
 * middle label" scheme deriveScale uses for numeric scales, reusable
 * for any other centered label set (letter-spacing's tight/normal/wide
 * being the other real case).
 */
export function pickCenteredLabels(labels: string[], centerLabel: string, count: number): string[] {
  const centerIndex = labels.indexOf(centerLabel)
  const start = centerIndex - Math.floor(count / 2)
  return Array.from({ length: count }, (_, i) => {
    const idx = start + i
    if (idx >= 0 && idx < labels.length) return labels[idx] as string
    // Real bug from code review: a single "+N" scheme for both overflow
    // directions produced malformed doubly-signed names ("+-13") when
    // idx went negative — below and above the label list now get
    // distinct, unambiguous names instead of arithmetic that assumed
    // idx was always non-negative.
    return idx < 0 ? `below${-idx}` : `above${idx - labels.length + 1}`
  })
}

/**
 * Turns a { pxValue: occurrenceCount } frequency map (from
 * parseCanvas.countPxValues, aggregated across canvases) into an
 * ordered token scale — every value that recurs at least `threshold`
 * times, ascending, named by size. This is extraction, not invention:
 * every token value is a real,
 * repeated value from the design reference; the threshold only filters
 * out one-off noise, it never substitutes a rounder-looking number.
 */
export function deriveScale(counts: Record<number, number>, threshold: number): ScaleToken[] {
  const kept = Object.entries(counts)
    .map(([px, count]) => ({ px: Number(px), count }))
    .filter((t) => t.count >= threshold)
    .sort((a, b) => a.px - b.px)

  const names = pickCenteredLabels(LABELS, 'md', kept.length)
  return kept.map((t, i) => ({ name: names[i] as string, px: t.px, count: t.count }))
}
