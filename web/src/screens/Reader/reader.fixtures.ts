// Minimal EPUB structure for Reader tests — served through the mocked
// content endpoint so foliate-js's epub.js parses a real spine + TOC.

export const CONTAINER_XML = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

export const CONTENT_OPF = `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="id">urn:uuid:test-book</dc:identifier>
    <dc:title>The Test Book</dc:title>
    <dc:language>en</dc:language>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="c1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
    <item id="c2" href="chapter2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="c1"/>
    <itemref idref="c2"/>
  </spine>
</package>`

export const NAV_XHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <body>
    <nav epub:type="toc">
      <ol>
        <li><a href="chapter1.xhtml">Chapter One</a></li>
        <li><a href="chapter2.xhtml">Chapter Two</a></li>
      </ol>
    </nav>
  </body>
</html>`

export const CHAPTER1_XHTML = `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><body><h1>Chapter One</h1><p>It begins.</p></body></html>`

/** A chapter carrying an injected script — the reader must never run it. */
export const CHAPTER1_HOSTILE = `<html><body><h1>Chapter One</h1><p>text</p>` +
  `<script>window.__pwned = true</script>` +
  `<img src="x" onerror="window.__pwned = true"></body></html>`
