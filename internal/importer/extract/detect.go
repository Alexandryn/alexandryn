package extract

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DetectFormat determines format from file content, never trusting extensions (backend-file-extractors.md FR-2).
func DetectFormat(f *os.File) (Format, error) {
	fi, err := f.Stat()
	if err != nil {
		return FormatUnknown, err
	}
	size := fi.Size()
	if size < 4 {
		return FormatUnknown, nil
	}

	// 1. Check PDF (%PDF-)
	pdfHeader := make([]byte, 5)
	if _, err := f.ReadAt(pdfHeader, 0); err == nil {
		if bytes.Equal(pdfHeader, []byte("%PDF-")) {
			return FormatPDF, nil
		}
	}

	// 2. Check physically first local zip file header for EPUB
	// Local file header structure:
	// 0..3: Signature PK\x03\x04
	// 8..9: Compression method (0 = Store)
	// 26..27: Filename length (8 = len("mimetype"))
	// 28..29: Extra field length
	// 30..30+fnLen: Filename
	// 30+fnLen+extraLen..: File data ("application/epub+zip")
	localHeader := make([]byte, 30)
	if _, err := f.ReadAt(localHeader, 0); err == nil && bytes.Equal(localHeader[:4], []byte{0x50, 0x4b, 0x03, 0x04}) {
		method := binary.LittleEndian.Uint16(localHeader[8:10])
		fnLen := binary.LittleEndian.Uint16(localHeader[26:28])
		extraLen := binary.LittleEndian.Uint16(localHeader[28:30])

		if method == 0 && fnLen == 8 {
			offset := int64(30)
			fnBuf := make([]byte, 8)
			if _, err := f.ReadAt(fnBuf, offset); err == nil && string(fnBuf) == "mimetype" {
				dataOffset := offset + 8 + int64(extraLen)
				mimeBuf := make([]byte, len("application/epub+zip"))
				if _, err := f.ReadAt(mimeBuf, dataOffset); err == nil && string(mimeBuf) == "application/epub+zip" {
					return FormatEPUB, nil
				}
			}
		}
	}

	// 3. Try parsing as zip archive (for CBZ / other)
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return FormatUnknown, nil
	}

	// Zip entry count cap (10,000 entries)
	if len(zr.File) > MaxZipEntryCount {
		return FormatUnknown, ErrTooManyEntries
	}

	var totalContent, imageCount int
	for _, entry := range zr.File {
		// Exclude __MACOSX and dotfiles
		name := entry.Name
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(filepath.Base(name), ".") {
			continue
		}
		if entry.FileInfo().IsDir() {
			continue
		}

		totalContent++

		// Sniff up to 512 bytes of entry content
		rc, err := entry.Open()
		if err != nil {
			continue
		}
		buf := make([]byte, 512)
		n, _ := io.ReadFull(rc, buf)
		_ = rc.Close()
		if n > 0 {
			ct := http.DetectContentType(buf[:n])
			if isImageContentType(ct) {
				imageCount++
			}
		}
	}

	if totalContent > 0 && float64(imageCount)/float64(totalContent) > 0.5 {
		return FormatCBZ, nil
	}

	return FormatUnknown, nil
}

func isImageContentType(ct string) bool {
	switch {
	case strings.HasPrefix(ct, "image/jpeg"),
		strings.HasPrefix(ct, "image/png"),
		strings.HasPrefix(ct, "image/gif"),
		strings.HasPrefix(ct, "image/webp"):
		return true
	default:
		return false
	}
}
