import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// frontend-accessibility.md FR-5: `prefers-contrast: more` swaps a
// subtle-contrast element to a higher-contrast variant. Playwright can't
// emulate prefers-contrast, so this proves the mechanism ships and is
// well-formed (maintainer decision G2). The reduced-motion half is proven
// live in e2e/a11y.app.spec.ts.

const cssDir = join(process.cwd(), 'src')
const a11yCss = readFileSync(join(cssDir, 'a11y.css'), 'utf8')
const themeCss = readFileSync(join(cssDir, 'theme.css'), 'utf8')

function hexVar(name: string): string {
  const match = new RegExp(`--color-${name}:\\s*(#[0-9a-fA-F]{6})`).exec(themeCss)
  if (!match) throw new Error(`theme.css has no --color-${name}`)
  return match[1] as string
}

function relLuminance(hex: string): number {
  const channels = [1, 3, 5].map((i) => {
    const c = Number.parseInt(hex.slice(i, i + 2), 16) / 255
    return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * channels[0]! + 0.7152 * channels[1]! + 0.0722 * channels[2]!
}

function ratio(a: string, b: string): number {
  const [hi, lo] = [relLuminance(a), relLuminance(b)].sort((x, y) => y - x)
  return (hi! + 0.05) / (lo! + 0.05)
}

describe('prefers-contrast: more adaptation (FR-5)', () => {
  it('a11y.css redefines --color-text-3 inside a prefers-contrast: more block', () => {
    const block =
      /@media\s*\(prefers-contrast:\s*more\)\s*\{[\s\S]*?--color-text-3:\s*([^;]+);/i.exec(a11yCss)
    expect(block).not.toBeNull()
    expect(block?.[1]?.trim()).toBe('var(--color-text-2)')
  })

  it('the high-contrast text-3 value passes WCAG AA on every surface', () => {
    const highContrastText3 = hexVar('text-2') // what text-3 resolves to under the override
    for (const surface of ['background', 'surface', 'surface-2', 'surface-3']) {
      expect(ratio(highContrastText3, hexVar(surface))).toBeGreaterThanOrEqual(4.5)
    }
  })
})
