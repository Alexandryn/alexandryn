package extract_test

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"context"
	"io"
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

// Defence 1: Zip bomb (highly compressed entry expanding to > 200 MiB)
// Must be aborted mid-stream with ErrOversized without exhausting memory.
func TestAdversarial_ZipBombRejectedWithErrOversized(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})

	wMime, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = wMime.Write([]byte("application/epub+zip"))

	wContainer, _ := zw.Create("META-INF/container.xml")
	_, _ = wContainer.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="content.opf"/></rootfiles>
</container>`))

	// Create an entry that compresses tiny but decompresses to 205 MiB of zeros
	wOpf, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "content.opf",
		Method: zip.Deflate,
	})
	if err != nil {
		t.Fatalf("create opf header: %v", err)
	}

	zeros := make([]byte, 10*1024*1024)
	for i := 0; i < 21; i++ {
		if _, err := wOpf.Write(zeros); err != nil {
			t.Fatalf("write zero chunk: %v", err)
		}
	}
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	_, err = extract.Extract(ctx, extract.FormatEPUB, tmp)
	if err != extract.ErrOversized {
		t.Fatalf("zip bomb error = %v, want ErrOversized", err)
	}
}

// Defence 2: Zip slip (entry named with directory traversal ../../etc/evil.opf)
// Structural test: extraction operates in memory, never constructs a file write.
func TestAdversarial_ZipSlipSafe(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	wMime, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = wMime.Write([]byte("application/epub+zip"))

	wContainer, _ := zw.Create("META-INF/container.xml")
	_, _ = wContainer.Write([]byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="../../etc/evil.opf"/></rootfiles>
</container>`))

	// Entry with traversal in name
	wEvil, _ := zw.Create("../../etc/evil.opf")
	_, _ = wEvil.Write([]byte(`<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Evil Traversal Book</dc:title>
  </metadata>
</package>`))
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	// Extraction operates purely in memory
	meta, err := extract.Extract(ctx, extract.FormatEPUB, tmp)
	if err != nil {
		t.Fatalf("Extract with traversal name: %v", err)
	}
	if meta.Title != "Evil Traversal Book" {
		t.Errorf("Title = %q, want Evil Traversal Book", meta.Title)
	}

	// Verify no file was created outside temp
	if _, err := os.Stat("/etc/evil.opf"); err == nil {
		t.Fatal("SECURITY VIOLATION: file written to /etc/evil.opf!")
	}
}

// Defence 3: Oversized raw stream (> 250 MiB)
// Spooling must abort immediately and return ErrOversized.
func TestAdversarial_RawStreamOversized(t *testing.T) {
	ctx := context.Background()
	oversizedReader := &zeroReader{}
	_, _, err := extract.Materialize(ctx, oversizedReader)
	if err != extract.ErrOversized {
		t.Fatalf("err = %v, want ErrOversized", err)
	}
}

// Defence 4: Too many entries (> 10,000 entries)
func TestAdversarial_TooManyZipEntries(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 10005; i++ {
		w, _ := zw.Create("entry.txt")
		_, _ = w.Write([]byte("a"))
	}
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	_, err := extract.Extract(ctx, extract.FormatCBZ, tmp)
	if err != extract.ErrTooManyEntries {
		t.Fatalf("err = %v, want ErrTooManyEntries", err)
	}
}

// Defence 5: Malformed container / XML entity bomb / unclosed tags
func TestAdversarial_MalformedContainerXmlPanicRecovery(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("META-INF/container.xml")
	_, _ = w.Write(append([]byte(`<?xml version="1.0"?><!DOCTYPE root [<!ENTITY b "boom">]><container>`), bytes.Repeat([]byte("<nested>"), 1000)...))
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	_, err := extract.Extract(ctx, extract.FormatEPUB, tmp)
	if err != extract.ErrMalformed {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}
