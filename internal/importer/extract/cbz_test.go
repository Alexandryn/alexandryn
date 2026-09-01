package extract_test

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

func TestExtractCBZ_WithComicInfo(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// Add ComicInfo.xml
	wInfo, _ := zw.Create("ComicInfo.xml")
	_, _ = wInfo.Write([]byte(`<?xml version="1.0"?>
<ComicInfo xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>Watchmen #1</Title>
  <Series>Watchmen</Series>
  <Number>1</Number>
  <Writer>Alan Moore, Dave Gibbons</Writer>
  <Publisher>DC Comics</Publisher>
  <Summary>Who watches the watchmen?</Summary>
</ComicInfo>`))

	// Add image pages
	coverBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x02}
	w01, _ := zw.Create("001.jpg")
	_, _ = w01.Write(coverBytes)

	w02, _ := zw.Create("002.jpg")
	_, _ = w02.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x03, 0x04})

	// Add __MACOSX and dotfile entries to prove they don't become the cover
	wMac, _ := zw.Create("__MACOSX/._000.jpg")
	_, _ = wMac.Write([]byte("fake-cover"))

	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	meta, err := extract.Extract(ctx, extract.FormatCBZ, tmp)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if meta.Title != "Watchmen #1" {
		t.Errorf("Title = %q, want Watchmen #1", meta.Title)
	}
	if len(meta.Authors) != 2 || meta.Authors[0] != "Alan Moore" || meta.Authors[1] != "Dave Gibbons" {
		t.Errorf("Authors = %+v, want [Alan Moore, Dave Gibbons]", meta.Authors)
	}
	if meta.Publisher == nil || *meta.Publisher != "DC Comics" {
		t.Errorf("Publisher = %v, want DC Comics", meta.Publisher)
	}
	if meta.Description == nil || *meta.Description != "Who watches the watchmen?" {
		t.Errorf("Description = %v, want Who watches the watchmen?", meta.Description)
	}
	if !bytes.Equal(meta.CoverBytes, coverBytes) {
		t.Errorf("CoverBytes = %v, want %v", meta.CoverBytes, coverBytes)
	}
}

func TestExtractCBZ_WithoutComicInfoReturnsErrNoTitle(t *testing.T) {
	ctx := context.Background()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	w01, _ := zw.Create("001.jpg")
	_, _ = w01.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00})
	_ = zw.Close()

	tmp := createTempFile(t, buf.Bytes())
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	_, err := extract.Extract(ctx, extract.FormatCBZ, tmp)
	if err != extract.ErrNoTitle {
		t.Fatalf("err = %v, want ErrNoTitle", err)
	}
}
