package extract

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"unicode/utf8"
)

type containerXML struct {
	XMLName   xml.Name `xml:"container"`
	Rootfiles []struct {
		FullPath string `xml:"full-path,attr"`
	} `xml:"rootfiles>rootfile"`
}

type opfPackageXML struct {
	XMLName  xml.Name `xml:"package"`
	Metadata struct {
		Titles       []string `xml:"title"`
		Creators     []string `xml:"creator"`
		Descriptions []string `xml:"description"`
		Publishers   []string `xml:"publisher"`
		Languages    []string `xml:"language"`
		Identifiers  []struct {
			Scheme string `xml:"scheme,attr"`
			ID     string `xml:"id,attr"`
			Value  string `xml:",chardata"`
		} `xml:"identifier"`
		Meta []struct {
			Name    string `xml:"name,attr"`
			Content string `xml:"content,attr"`
		} `xml:"meta"`
	} `xml:"metadata"`
	Manifest struct {
		Items []struct {
			ID         string `xml:"id,attr"`
			Href       string `xml:"href,attr"`
			MediaType  string `xml:"media-type,attr"`
			Properties string `xml:"properties,attr"`
		} `xml:"item"`
	} `xml:"manifest"`
}

// ExtractEPUB extracts metadata and cover from an EPUB container (backend-file-extractors.md FR-9).
func ExtractEPUB(ctx context.Context, f *os.File) (meta ExtractedMetadata, err error) {
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

	entryMap := make(map[string]*zip.File, len(zr.File))
	for _, entry := range zr.File {
		if !utf8.ValidString(entry.Name) {
			continue
		}
		entryMap[entry.Name] = entry
	}

	var bytesRead int64
	readEntry := func(entry *zip.File, maxBytes int64) ([]byte, error) {
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()

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

	// 1. Read META-INF/container.xml
	containerEntry, ok := entryMap["META-INF/container.xml"]
	if !ok {
		return ExtractedMetadata{}, ErrMalformed
	}
	containerBytes, err := readEntry(containerEntry, 1*1024*1024)
	if err != nil {
		if errorsIs(err, ErrOversized) {
			return ExtractedMetadata{}, ErrOversized
		}
		return ExtractedMetadata{}, ErrMalformed
	}

	var container containerXML
	if err := xml.Unmarshal(containerBytes, &container); err != nil {
		return ExtractedMetadata{}, ErrMalformed
	}
	if len(container.Rootfiles) == 0 || strings.TrimSpace(container.Rootfiles[0].FullPath) == "" {
		return ExtractedMetadata{}, ErrMalformed
	}

	opfPath := strings.TrimSpace(container.Rootfiles[0].FullPath)
	opfEntry, ok := entryMap[opfPath]
	if !ok {
		// Try clean path
		opfEntry, ok = entryMap[path.Clean(opfPath)]
		if !ok {
			return ExtractedMetadata{}, ErrMalformed
		}
	}

	// 2. Read OPF file
	opfBytes, err := readEntry(opfEntry, 5*1024*1024)
	if err != nil {
		if errorsIs(err, ErrOversized) {
			return ExtractedMetadata{}, ErrOversized
		}
		return ExtractedMetadata{}, ErrMalformed
	}

	var opf opfPackageXML
	if err := xml.Unmarshal(opfBytes, &opf); err != nil {
		return ExtractedMetadata{}, ErrMalformed
	}

	// 3. Extract Metadata fields
	var rawTitle string
	if len(opf.Metadata.Titles) > 0 {
		rawTitle = opf.Metadata.Titles[0]
	}

	var authors []string
	for _, c := range opf.Metadata.Creators {
		if strings.TrimSpace(c) != "" {
			authors = append(authors, c)
		}
	}

	var isbn *string
	for _, ident := range opf.Metadata.Identifiers {
		val := strings.TrimSpace(ident.Value)
		scheme := strings.ToUpper(ident.Scheme)
		if scheme == "ISBN" || strings.HasPrefix(strings.ToLower(val), "urn:isbn:") {
			val = strings.TrimPrefix(val, "urn:isbn:")
			val = strings.TrimPrefix(val, "URN:ISBN:")
			val = strings.TrimSpace(val)
			if isbn == nil && val != "" {
				isbn = &val
			}
		} else if isbn == nil && (len(strings.ReplaceAll(val, "-", "")) == 10 || len(strings.ReplaceAll(val, "-", "")) == 13) {
			isbn = &val
		}
	}

	var language *string
	if len(opf.Metadata.Languages) > 0 && strings.TrimSpace(opf.Metadata.Languages[0]) != "" {
		l := strings.TrimSpace(opf.Metadata.Languages[0])
		language = &l
	}

	var publisher *string
	if len(opf.Metadata.Publishers) > 0 && strings.TrimSpace(opf.Metadata.Publishers[0]) != "" {
		p := strings.TrimSpace(opf.Metadata.Publishers[0])
		publisher = &p
	}

	var description *string
	if len(opf.Metadata.Descriptions) > 0 && strings.TrimSpace(opf.Metadata.Descriptions[0]) != "" {
		d := strings.TrimSpace(opf.Metadata.Descriptions[0])
		description = &d
	}

	// 4. Resolve Cover image
	var coverHref string

	// Try EPUB3 properties="cover-image"
	for _, item := range opf.Manifest.Items {
		if strings.Contains(item.Properties, "cover-image") {
			coverHref = item.Href
			break
		}
	}

	// Try EPUB2 <meta name="cover" content="item-id">
	if coverHref == "" {
		var coverItemID string
		for _, m := range opf.Metadata.Meta {
			if strings.EqualFold(m.Name, "cover") {
				coverItemID = m.Content
				break
			}
		}
		if coverItemID != "" {
			for _, item := range opf.Manifest.Items {
				if item.ID == coverItemID {
					coverHref = item.Href
					break
				}
			}
		}
	}

	var coverBytes []byte
	if coverHref != "" {
		coverPath := path.Join(path.Dir(opfPath), coverHref)
		if coverEntry, ok := entryMap[coverPath]; ok {
			cb, err := readEntry(coverEntry, MaxCoverBytes)
			if err == nil {
				coverBytes = cb
			}
		}
	}

	return NewExtractedMetadata(rawTitle, authors, isbn, language, publisher, description, coverBytes, FormatEPUB)
}

func errorsIs(err, target error) bool {
	return err == target || strings.Contains(fmt.Sprint(err), fmt.Sprint(target))
}
