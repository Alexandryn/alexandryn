// Minimal ambient types for the vendored foliate-js `epub.js` — only the
// surface the reader (`web/src/screens/Reader`) actually uses. The full
// module (ADR 0023) is vendored verbatim from foliate-js 1.0.1; these
// declarations are ours.

export interface EpubLoader {
  loadText: (uri: string) => Promise<string>
  loadBlob: (uri: string) => Promise<Blob>
  getSize: (uri: string) => number
  sha1?: (data: unknown) => Promise<string>
}

export interface EpubSection {
  /** The spine item's href, relative to the EPUB root (the zip path). */
  id: string
  /** The spine-step CFI for this section (foliate-generated). */
  cfi: string
  linear?: string
  resolveHref: (href: string) => string
}

export interface EpubTocItem {
  label: string
  href: string
  subitems?: EpubTocItem[]
}

export interface EpubMetadata {
  title?: string | { main?: string }
  [key: string]: unknown
}

export class EPUB {
  constructor(loader: EpubLoader)
  init(): Promise<this>
  sections: EpubSection[]
  toc?: EpubTocItem[]
  metadata?: EpubMetadata
  dir?: string
  resolveHref(href: string): { index: number; anchor: (doc: Document) => Range | Element } | null
}
