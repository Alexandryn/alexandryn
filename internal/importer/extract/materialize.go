package extract

import (
	"context"
	"fmt"
	"io"
	"os"
)

// Materialize spools r into a temporary file capped at 250 MiB (backend-file-extractors.md FR-1).
// It returns the open *os.File positioned at offset 0, and a cleanup function that closes
// and removes the temp file. If the stream exceeds 250 MiB, it aborts immediately and returns ErrOversized.
func Materialize(ctx context.Context, r io.Reader) (*os.File, func(), error) {
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}

	tmp, err := os.CreateTemp("", "alexandryn-import-*")
	if err != nil {
		return nil, nil, fmt.Errorf("creating temp file for import spooling: %w", err)
	}

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}

	// Read in 64 KiB chunks to check context cancellation and enforce raw limit
	buf := make([]byte, 64*1024)
	var total int64

	for {
		if ctx.Err() != nil {
			cleanup()
			return nil, nil, ctx.Err()
		}

		n, rErr := r.Read(buf)
		if n > 0 {
			total += int64(n)
			if total > MaxRawSpoolBytes {
				cleanup()
				return nil, nil, ErrOversized
			}
			if _, wErr := tmp.Write(buf[:n]); wErr != nil {
				cleanup()
				return nil, nil, fmt.Errorf("writing to import temp file: %w", wErr)
			}
		}

		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			cleanup()
			return nil, nil, fmt.Errorf("reading source stream: %w", rErr)
		}
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("seeking to start of import temp file: %w", err)
	}

	return tmp, cleanup, nil
}
