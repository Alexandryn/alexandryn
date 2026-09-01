package extract

import (
	"context"
	"os"
)

// ExtractPDF extracts metadata from a PDF file (backend-file-extractors.md FR-11).
func ExtractPDF(ctx context.Context, f *os.File) (ExtractedMetadata, error) {
	return ExtractedMetadata{}, nil
}
