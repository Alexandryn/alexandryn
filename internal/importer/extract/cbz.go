package extract

import (
	"context"
	"os"
)

// ExtractCBZ extracts metadata and cover from a CBZ comic archive (backend-file-extractors.md FR-10).
func ExtractCBZ(ctx context.Context, f *os.File) (ExtractedMetadata, error) {
	return ExtractedMetadata{}, nil
}
