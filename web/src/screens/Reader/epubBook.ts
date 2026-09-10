// Loads an owned EPUB's structure through the sanitised content endpoint
// (frontend-reader.md FR-1) — foliate-js's own `epub.js` parses
// container.xml / OPF / nav, so spine order, TOC, and the spine-step
// CFIs come from the library, never hand-derived. No `blob:` URL is ever
// built: chapter documents load by pointing the <iframe> at the content
// endpoint directly (FR-1's blob-avoidance architecture).

import { ApiError, getText } from '../../data/http'
import { EPUB, type EpubSection, type EpubTocItem } from '../../vendor/foliate/epub'

/** The content endpoint URL for one entry inside an Edition's EPUB. */
export function contentUrl(editionId: string, path: string): string {
  const clean = path.replace(/^\/+/, '')
  return `/api/v1/library/editions/${encodeURIComponent(editionId)}/reader/content/${clean
    .split('/')
    .map(encodeURIComponent)
    .join('/')}`
}

export interface LoadedBook {
  sections: EpubSection[]
  toc: EpubTocItem[]
  title: string
}

/**
 * Fetches an EPUB entry as text. A 404 becomes an empty string — foliate
 * probes for optional files (encryption.xml, display-options, an NCX)
 * whose absence is normal, and the content endpoint 404s a missing entry.
 */
async function loadEpubText(editionId: string, uri: string, signal?: AbortSignal): Promise<string> {
  try {
    return await getText(contentUrl(editionId, uri), { signal })
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) return ''
    throw err
  }
}

export async function loadEpub(editionId: string, signal?: AbortSignal): Promise<LoadedBook> {
  const book = new EPUB({
    loadText: (uri) => loadEpubText(editionId, uri, signal),
    // The reader never asks foliate for blobs — chapter documents load
    // via the <iframe> src, and images resolve as relative URLs against
    // it. A rejected loadBlob keeps foliate from constructing blob: URLs.
    loadBlob: () => Promise.reject(new Error('blob loading is disabled in the reader')),
    getSize: () => 0,
    sha1: async () => '',
  })

  await book.init()

  const meta = book.metadata?.title
  const title =
    typeof meta === 'string' ? meta : (meta && typeof meta === 'object' && meta.main) || 'Untitled'

  return {
    sections: book.sections.filter((s) => s.linear !== 'no'),
    toc: book.toc ?? [],
    title,
  }
}

/** Flattens a TOC tree to a list with depth, for an accessible nav list. */
export interface FlatTocEntry {
  label: string
  href: string
  depth: number
}

export function flattenToc(items: EpubTocItem[], depth = 0): FlatTocEntry[] {
  return items.flatMap((item) => [
    { label: item.label, href: item.href, depth },
    ...(item.subitems ? flattenToc(item.subitems, depth + 1) : []),
  ])
}

/** Normalises an EPUB path for comparison (strips fragments/queries, resolves separators). */
function normalisePath(p: string): string {
  const clean = p.split(/[?#]/)[0] ?? ''
  try {
    return decodeURIComponent(clean).replace(/^\/+/, '').replace(/\/+/g, '/')
  } catch {
    return clean.replace(/^\/+/, '').replace(/\/+/g, '/')
  }
}

/** The spine index whose section href matches a TOC href (ignoring a #fragment). */
export function sectionIndexForHref(sections: EpubSection[], href: string): number {
  const path = normalisePath(href)
  if (!path) return -1
  return sections.findIndex((s) => {
    const sPath = normalisePath(s.id)
    return sPath === path || sPath.endsWith('/' + path) || path.endsWith('/' + sPath)
  })
}
