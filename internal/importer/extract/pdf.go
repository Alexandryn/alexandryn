package extract

import (
	"context"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// ExtractPDF extracts metadata from a PDF file using pdfcpu (backend-file-extractors.md FR-11).
func ExtractPDF(ctx context.Context, f *os.File) (meta ExtractedMetadata, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrMalformed
		}
	}()

	if ctx.Err() != nil {
		return ExtractedMetadata{}, ctx.Err()
	}

	if _, err := f.Seek(0, 0); err != nil {
		return ExtractedMetadata{}, ErrMalformed
	}

	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	info, infoErr := api.PDFInfo(f, "", nil, false, conf)
	if infoErr != nil {
		return ExtractedMetadata{}, ErrMalformed
	}

	var rawTitle string
	if info.Title != "" {
		rawTitle = info.Title
	}

	var authors []string
	if strings.TrimSpace(info.Author) != "" {
		for _, a := range strings.Split(info.Author, ";") {
			for _, part := range strings.Split(a, ",") {
				clean := strings.TrimSpace(part)
				if clean != "" {
					authors = append(authors, clean)
				}
			}
		}
	}

	var description *string
	if strings.TrimSpace(info.Subject) != "" {
		d := strings.TrimSpace(info.Subject)
		description = &d
	} else if len(info.Keywords) > 0 {
		d := strings.TrimSpace(strings.Join(info.Keywords, ", "))
		if d != "" {
			description = &d
		}
	}

	return NewExtractedMetadata(rawTitle, authors, nil, nil, nil, description, nil, FormatPDF)
}
