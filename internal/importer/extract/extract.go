package extract

import (
	"context"
	"fmt"
	"os"
)

// Extract parses metadata and cover image from f according to format.
// Execution is bounded by the context deadline and decompressed read limit.
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
