package extract_test

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

func TestDetectFormat_PDF(t *testing.T) {
	tmp := createTempFile(t, []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n"))

	format, err := extract.DetectFormat(tmp)
	if err != nil {
		t.Fatalf("DetectFormat: %v", err)
	}
	if format != extract.FormatPDF {
		t.Fatalf("format = %v, want pdf", format)
	}
}

func TestDetectFormat_EPUB(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// EPUB requires physically first local header: mimetype, Store method, content application/epub+zip
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		t.Fatalf("create mimetype header: %v", err)
	}
	if _, err := w.Write([]byte("application/epub+zip")); err != nil {
		t.Fatalf("write mimetype content: %v", err)
	}

	w2, err := zw.Create("META-INF/container.xml")
	if err != nil {
		t.Fatalf("create container.xml: %v", err)
	}
	_, _ = w2.Write([]byte("<container/>"))
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	format, err := extract.DetectFormat(tmp)
	if err != nil {
		t.Fatalf("DetectFormat: %v", err)
	}
	if format != extract.FormatEPUB {
		t.Fatalf("format = %v, want epub", format)
	}
}

func TestDetectFormat_CBZ(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Valid JPEG magic bytes for page 1 & 2
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

	w1, _ := zw.Create("page01.jpg")
	_, _ = w1.Write(jpegHeader)

	w2, _ := zw.Create("page02.jpg")
	_, _ = w2.Write(jpegHeader)

	// Include __MACOSX and dotfile entries to prove they are excluded from majority count
	wMac, _ := zw.Create("__MACOSX/._page01.jpg")
	_, _ = wMac.Write([]byte("resource-fork-placeholder"))

	wDot, _ := zw.Create(".DS_Store")
	_, _ = wDot.Write([]byte("junk"))

	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	format, err := extract.DetectFormat(tmp)
	if err != nil {
		t.Fatalf("DetectFormat: %v", err)
	}
	if format != extract.FormatCBZ {
		t.Fatalf("format = %v, want cbz", format)
	}
}

func TestDetectFormat_TooManyEntries(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for i := 0; i < 10001; i++ {
		w, _ := zw.Create(filepath.Join("folder", "file.txt"))
		_, _ = w.Write([]byte("x"))
	}
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())

	_, err := extract.DetectFormat(tmp)
	if err != extract.ErrTooManyEntries {
		t.Fatalf("err = %v, want ErrTooManyEntries", err)
	}
}

func TestDetectFormat_Unknown(t *testing.T) {
	tmp := createTempFile(t, []byte("random text content not a zip or pdf"))

	format, err := extract.DetectFormat(tmp)
	if err != nil {
		t.Fatalf("DetectFormat: %v", err)
	}
	if format != extract.FormatUnknown {
		t.Fatalf("format = %v, want unknown", format)
	}
}

func createTempFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	tmp, err := os.CreateTemp("", "detect-test-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	t.Cleanup(func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	})
	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	return tmp
}
