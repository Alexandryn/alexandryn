/**
 * Extracts every `--name:value` custom property declared on the
 * `ref="{{ rootRef }}"` div's inline style attribute — the one place a
 * `.design-reference/*.dc.html` canvas declares real CSS custom
 * properties (confirmed by inspection: no `:root { }` block, no
 * `@media` query, anywhere in any of the four canvases).
 */
export function parseRootCustomProperties(html: string): Record<string, string> {
  const match = html.match(/ref="\{\{ rootRef \}\}"\s+style="([^"]*)"/)
  if (!match?.[1]) {
    throw new Error('parseRootCustomProperties: no ref="{{ rootRef }}" style attribute found')
  }
  const style = match[1]
  const props: Record<string, string> = {}
  // Split on ';' that separates declarations — custom-property values here
  // never themselves contain a literal ';' (shadows/colors/lengths only).
  for (const decl of style.split(';')) {
    const propMatch = decl.match(/^(--[a-z0-9-]+):(.+)$/)
    if (propMatch?.[1] && propMatch[2] !== undefined) {
      props[propMatch[1]] = propMatch[2]
    }
  }
  return props
}

/**
 * Counts how many times each distinct px value appears for a given CSS
 * property (e.g. `border-radius`, `gap`, `font-size`) across a whole
 * canvas file — the raw frequency data a spacing/radius/typography
 * scale gets derived from (frontend-design-tokens.md FR-1: extracted
 * from real layout rules, never invented).
 *
 * Matches decimal values too (`(\d+(?:\.\d+)?)`), not just integers — a
 * real bug code review caught: an earlier integer-only version silently
 * dropped every half-pixel font-size declaration, and `12.5px` turns out
 * to be the single most-frequent font-size value across all four
 * canvases (185 occurrences, ahead of any integer value), so the
 * generated type scale was built on incomplete data with no error or
 * warning — it just quietly extracted the wrong scale.
 */
export function countPxValues(html: string, property: string): Record<number, number> {
  const pattern = new RegExp(`${property}:(\\d+(?:\\.\\d+)?)px`, 'g')
  const counts: Record<number, number> = {}
  for (const match of html.matchAll(pattern)) {
    const value = Number(match[1])
    counts[value] = (counts[value] ?? 0) + 1
  }
  return counts
}

/** Same idea as countPxValues, for `em`-unit properties (letter-spacing). */
export function countEmValues(html: string, property: string): Record<string, number> {
  const pattern = new RegExp(`${property}:(-?\\.?\\d+(?:\\.\\d+)?)em`, 'g')
  const counts: Record<string, number> = {}
  for (const match of html.matchAll(pattern)) {
    const value = match[1] as string
    counts[value] = (counts[value] ?? 0) + 1
  }
  return counts
}
