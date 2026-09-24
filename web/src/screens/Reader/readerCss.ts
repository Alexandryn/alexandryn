// Applies the reader's own typography CSS to a chapter document.
//
// The content endpoint's CSP (default-src 'self', no 'unsafe-inline')
// blocks every inline <style>, so a <style> element injected into the
// chapter never applied. A constructed stylesheet adopted into the
// document is not an inline style, so it applies without loosening the
// CSP. The sheet must come from the chapter's own window: a document only
// adopts sheets its own realm constructed.

const sheets = new WeakMap<Document, CSSStyleSheet>()

/** Replaces the reader CSS in `doc`; false where the browser cannot adopt sheets. */
export function applyReaderCss(doc: Document, css: string): boolean {
  const win = doc.defaultView as (Window & typeof globalThis) | null
  if (!win || typeof win.CSSStyleSheet !== 'function' || !('adoptedStyleSheets' in doc)) {
    return false
  }
  let sheet = sheets.get(doc)
  if (!sheet) {
    sheet = new win.CSSStyleSheet()
    doc.adoptedStyleSheets = [...doc.adoptedStyleSheets, sheet]
    sheets.set(doc, sheet)
  }
  sheet.replaceSync(css)
  return true
}
