import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

// frontend-accessibility.md FR-5: `prefers-contrast: more` swaps a
// subtle-contrast element to a higher-contrast variant. Playwright can't
// emulate prefers-contrast (maintainer decision G2), so this asserts the
// mechanism is well-formed. That the resolved value actually passes WCAG
// AA on every surface is checked by `npm run tokens:check-contrast` (a CI
// step, using the shared contrast helpers) — not re-derived here.
// The reduced-motion half is proven live in e2e/a11y-gallery.gallery.spec.ts.

const a11yCss = readFileSync(join(process.cwd(), 'src', 'a11y.css'), 'utf8')

describe('prefers-contrast: more adaptation (FR-5)', () => {
  it('redefines --color-text-3 to the text-2 token inside a prefers-contrast: more block', () => {
    const block =
      /@media\s*\(prefers-contrast:\s*more\)\s*\{[\s\S]*?--color-text-3:\s*([^;]+);/i.exec(a11yCss)
    expect(block).not.toBeNull()
    expect(block?.[1]?.trim()).toBe('var(--color-text-2)')
  })

  it('is loaded by the app entry point after the token file', () => {
    const mainTsx = readFileSync(join(process.cwd(), 'src', 'main.tsx'), 'utf8')
    const themeIdx = mainTsx.indexOf("'./theme.css'")
    const a11yIdx = mainTsx.indexOf("'./a11y.css'")
    expect(themeIdx).toBeGreaterThanOrEqual(0)
    expect(a11yIdx).toBeGreaterThan(themeIdx) // source order = the override wins the cascade
  })
})
