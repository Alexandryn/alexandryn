package extract_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

func TestExtractEPUB_ValidEPUB2And3(t *testing.T) {
	ctx := context.Background()

	// Build a valid synthetic EPUB archive
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// mimetype (Store)
	wMime, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = wMime.Write([]byte("application/epub+zip"))

	// META-INF/container.xml
	wContainer, _ := zw.Create("META-INF/container.xml")
	_, _ = wContainer.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	// OEBPS/content.opf
	wOpf, _ := zw.Create("OEBPS/content.opf")
	_, _ = wOpf.Write([]byte(`<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Dune</dc:title>
    <dc:creator>Frank Herbert</dc:creator>
    <dc:identifier id="pub-id" opf:scheme="ISBN">9780441172719</dc:identifier>
    <dc:language>en</dc:language>
    <dc:publisher>Chilton Books</dc:publisher>
    <dc:description>Set on the desert planet Arrakis...</dc:description>
  </metadata>
  <manifest>
    <item id="cover-img" href="images/cover.jpg" media-type="image/jpeg" properties="cover-image"/>
    <item id="chapter1" href="chapter1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine>
    <itemref idref="chapter1"/>
  </spine>
</package>`))

	// Cover image
	coverData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0xAA, 0xBB}
	wCover, _ := zw.Create("OEBPS/images/cover.jpg")
	_, _ = wCover.Write(coverData)

	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	meta, err := extract.Extract(ctx, extract.FormatEPUB, tmp)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if meta.Title != "Dune" {
		t.Errorf("Title = %q, want Dune", meta.Title)
	}
	if len(meta.Authors) != 1 || meta.Authors[0] != "Frank Herbert" {
		t.Errorf("Authors = %+v, want [Frank Herbert]", meta.Authors)
	}
	if meta.ISBN == nil || *meta.ISBN != "9780441172719" {
		t.Errorf("ISBN = %v, want 9780441172719", meta.ISBN)
	}
	if meta.Language == nil || *meta.Language != "en" {
		t.Errorf("Language = %v, want en", meta.Language)
	}
	if meta.Publisher == nil || *meta.Publisher != "Chilton Books" {
		t.Errorf("Publisher = %v, want Chilton Books", meta.Publisher)
	}
	if len(meta.CoverBytes) != len(coverData) {
		t.Errorf("CoverBytes len = %d, want %d", len(meta.CoverBytes), len(coverData))
	}
}

func TestExtractEPUB_MissingTitleIsErrNoTitle(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	wMime, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = wMime.Write([]byte("application/epub+zip"))

	wContainer, _ := zw.Create("META-INF/container.xml")
	_, _ = wContainer.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`))

	wOpf, _ := zw.Create("content.opf")
	_, _ = wOpf.Write([]byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>   </dc:title>
    <dc:creator>Some Author</dc:creator>
  </metadata>
</package>`))
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	_, err := extract.Extract(ctx, extract.FormatEPUB, tmp)
	if err != extract.ErrNoTitle {
		t.Fatalf("err = %v, want ErrNoTitle", err)
	}
}

func TestExtractEPUB_MalformedXMLIsErrMalformed(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	wMime, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = wMime.Write([]byte("application/epub+zip"))

	wContainer, _ := zw.Create("META-INF/container.xml")
	_, _ = wContainer.Write([]byte(`not xml at all <<< >>>`))
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	_, err := extract.Extract(ctx, extract.FormatEPUB, tmp)
	if err != extract.ErrMalformed {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}
