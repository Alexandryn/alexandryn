/** Parses a #RRGGBB hex color into [r,g,b], each 0-255. */
function hexToRgb(hex: string): [number, number, number] {
  const clean = hex.replace('#', '')
  return [
    Number.parseInt(clean.slice(0, 2), 16),
    Number.parseInt(clean.slice(2, 4), 16),
    Number.parseInt(clean.slice(4, 6), 16),
  ]
}

/** WCAG 2.x relative luminance. */
function relativeLuminance(hex: string): number {
  const [r, g, b] = hexToRgb(hex).map((c) => {
    const s = c / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * (r as number) + 0.7152 * (g as number) + 0.0722 * (b as number)
}

/** WCAG 2.x contrast ratio between two colors, always ≥1 regardless of argument order. */
export function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(a)
  const lb = relativeLuminance(b)
  const lighter = Math.max(la, lb)
  const darker = Math.min(la, lb)
  return (lighter + 0.05) / (darker + 0.05)
}

/** WCAG AA for normal text: contrast ratio ≥ 4.5:1. */
export function meetsWcagAA(a: string, b: string): boolean {
  return contrastRatio(a, b) >= 4.5
}
