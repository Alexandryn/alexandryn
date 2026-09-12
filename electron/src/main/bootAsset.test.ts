import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { syncTokensPlugin } from '../../electron.vite.config'

// Boot asset & token sync.
// Asserts tokens.css existence, valid CSS contents, and build-time plugin assertion.

describe('boot asset token sync', () => {
  const webTokensPath = resolve(import.meta.dirname, '../../../web/src/tokens.css')
  const bootTokensPath = resolve(import.meta.dirname, '../renderer/boot/tokens.css')
  const bootHtmlPath = resolve(import.meta.dirname, '../renderer/boot/index.html')
  const bootCssPath = resolve(import.meta.dirname, '../renderer/boot/boot.css')

  it('web/src/tokens.css exists as the canonical token source', () => {
    expect(existsSync(webTokensPath)).toBe(true)
  })

  it('boot tokens.css exists and matches web/src/tokens.css', () => {
    expect(existsSync(bootTokensPath)).toBe(true)
    const webTokens = readFileSync(webTokensPath, 'utf8')
    const bootTokens = readFileSync(bootTokensPath, 'utf8')
    expect(bootTokens).toBe(webTokens)
  })

  it('boot index.html includes semantic status region, aria-live polite, and loading/error states', () => {
    const html = readFileSync(bootHtmlPath, 'utf8')
    expect(html).toContain('role="status"')
    expect(html).toContain('aria-live="polite"')
    expect(html).toContain('id="loading-state"')
    expect(html).toContain('id="error-state"')
    expect(html).toContain('id="retry-btn"')
  })

  it('boot boot.css imports tokens.css and contains focus-visible ring styles', () => {
    const css = readFileSync(bootCssPath, 'utf8')
    expect(css).toContain("@import './tokens.css';")
    expect(css).toContain(':focus-visible')
    expect(css).toContain('outline:')
  })

  it('syncTokensPlugin has buildStart hook that validates web tokens existence', () => {
    const plugin = syncTokensPlugin()
    expect(plugin.name).toBe('assert-and-sync-tokens')
    expect(typeof plugin.buildStart).toBe('function')
    expect(() => plugin.buildStart()).not.toThrow()
  })
})
