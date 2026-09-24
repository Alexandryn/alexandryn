import { describe, expect, it } from 'vitest'

import { applyReaderCss } from './readerCss'

class FakeSheet {
  css = ''
  replaceSync(css: string) {
    this.css = css
  }
}

function fakeDoc(existing: unknown[] = []) {
  return {
    defaultView: { CSSStyleSheet: FakeSheet },
    adoptedStyleSheets: existing,
  } as unknown as Document & { adoptedStyleSheets: FakeSheet[] }
}

describe('applyReaderCss', () => {
  it('adopts one sheet from the chapter window and replaces its CSS on each call', () => {
    const bookSheet = new FakeSheet()
    const doc = fakeDoc([bookSheet])
    expect(applyReaderCss(doc, 'body{font-size:18px}')).toBe(true)
    expect(applyReaderCss(doc, 'body{font-size:20px}')).toBe(true)
    expect(doc.adoptedStyleSheets).toHaveLength(2)
    expect(doc.adoptedStyleSheets[0]).toBe(bookSheet)
    expect(doc.adoptedStyleSheets[1]).toBeInstanceOf(FakeSheet)
    expect(doc.adoptedStyleSheets[1]?.css).toBe('body{font-size:20px}')
  })

  it('reports false where the document cannot adopt sheets', () => {
    expect(applyReaderCss({ defaultView: null } as unknown as Document, 'x{}')).toBe(false)
  })
})
