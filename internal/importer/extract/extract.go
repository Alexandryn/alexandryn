package extract

import (
	"context"
	"fmt"
	"os"
)

// Extract parses metadata and cover image from f according to format (backend-file-extractors.md FR-3).
// Execution is bounded by the 30-second context deadline and 200 MiB decompressed limit.
func Extract(ctx context.Context, format Format, f *os.File) (ExtractedMetadata, error) {
	if ctx.Err() != nil {
		return ExtractedMetadata{}, ctx.Err()
	}

	switch format {
	case FormatEPUB:
		return ExtractEPUB(ctx, f)
	case FormatCBZ:
		return ExtractCBZ(ctx, f)
	case FormatPDF:
		return ExtractPDF(ctx, f)
	default:
		return ExtractedMetadata{}, fmt.Errorf("%w: unsupported format %q", ErrMalformed, format)
	}
}
