package extract

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type comicInfoXML struct {
	XMLName   xml.Name `xml:"ComicInfo"`
	Title     string   `xml:"Title"`
	Series    string   `xml:"Series"`
	Number    string   `xml:"Number"`
	Writer    string   `xml:"Writer"`
	Publisher string   `xml:"Publisher"`
	Summary   string   `xml:"Summary"`
	Notes     string   `xml:"Notes"`
}

// ExtractCBZ extracts metadata and cover from a CBZ comic archive (backend-file-extractors.md FR-10).
func ExtractCBZ(ctx context.Context, f *os.File) (meta ExtractedMetadata, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrMalformed
		}
	}()

	fi, statErr := f.Stat()
	if statErr != nil {
		return ExtractedMetadata{}, statErr
	}
	size := fi.Size()

	zr, zipErr := zip.NewReader(f, size)
	if zipErr != nil {
		return ExtractedMetadata{}, ErrMalformed
	}
	if len(zr.File) > MaxZipEntryCount {
		return ExtractedMetadata{}, ErrTooManyEntries
	}

	var comicInfoEntry *zip.File
	var imageEntries []*zip.File
	var bytesRead int64

	readEntry := func(entry *zip.File, maxBytes int64) ([]byte, error) {
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()

		lr := io.LimitReader(rc, maxBytes+1)
		data, err := io.ReadAll(lr)
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxBytes {
			return nil, ErrOversized
		}

		bytesRead += int64(len(data))
		if bytesRead > MaxDecompressedReadBytes {
			return nil, ErrOversized
		}
		return data, nil
	}

	for _, entry := range zr.File {
		name := entry.Name
		// Exclude __MACOSX and dotfile entries
		if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(filepath.Base(name), ".") {
			continue
		}
		// Non-UTF-8 entry names are excluded from the page list (FR-10, review 0049 finding 3)
		if !utf8.ValidString(name) {
			continue
		}
		if entry.FileInfo().IsDir() {
			continue
		}

		if strings.EqualFold(filepath.Base(name), "comicinfo.xml") {
			comicInfoEntry = entry
			continue
		}

		// Sniff for image entry
		rc, err := entry.Open()
		if err != nil {
			continue
		}
		buf := make([]byte, 512)
		n, _ := io.ReadFull(rc, buf)
		_ = rc.Close()
		if n > 0 && isImageContentType(http.DetectContentType(buf[:n])) {
			imageEntries = append(imageEntries, entry)
		}
	}

	var rawTitle string
	var authors []string
	var publisher *string
	var description *string

	if comicInfoEntry != nil {
		infoBytes, err := readEntry(comicInfoEntry, 5*1024*1024)
		if err == nil {
			var ci comicInfoXML
			if err := xml.Unmarshal(infoBytes, &ci); err == nil {
				rawTitle = strings.TrimSpace(ci.Title)
				if rawTitle == "" && strings.TrimSpace(ci.Series) != "" {
					rawTitle = strings.TrimSpace(ci.Series)
					if strings.TrimSpace(ci.Number) != "" {
						rawTitle += " #" + strings.TrimSpace(ci.Number)
					}
				}
				if strings.TrimSpace(ci.Writer) != "" {
					for _, a := range strings.Split(ci.Writer, ",") {
						if clean := strings.TrimSpace(a); clean != "" {
							authors = append(authors, clean)
						}
					}
				}
				if strings.TrimSpace(ci.Publisher) != "" {
					p := strings.TrimSpace(ci.Publisher)
					publisher = &p
				}
				if strings.TrimSpace(ci.Summary) != "" {
					d := strings.TrimSpace(ci.Summary)
					description = &d
				} else if strings.TrimSpace(ci.Notes) != "" {
					d := strings.TrimSpace(ci.Notes)
					description = &d
				}
			}
		}
	}

	// First image in filename-sorted order as cover
	var coverBytes []byte
	if len(imageEntries) > 0 {
		sort.Slice(imageEntries, func(i, j int) bool {
			return imageEntries[i].Name < imageEntries[j].Name
		})

		cb, err := readEntry(imageEntries[0], MaxCoverBytes)
		if err == nil {
			coverBytes = cb
		}
	}

	return NewExtractedMetadata(rawTitle, authors, nil, nil, publisher, description, coverBytes, FormatCBZ)
}
