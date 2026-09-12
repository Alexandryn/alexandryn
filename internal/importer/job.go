package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// ImportJobPayload carries the arguments for an import execution job.
type ImportJobPayload struct {
	CandidateID   string               `json:"candidateId"`
	SourceID      string               `json:"sourceId"`
	FileReference domain.FileReference `json:"fileReference"`
}

// SourceResolver resolves a file reference from a registered source.
type SourceResolver interface {
	Resolve(ctx context.Context, sourceID string, fileRef domain.FileReference) (io.ReadCloser, error)
}

// NewJobHandler constructs the job runner handler for kind "import".
func NewJobHandler(
	resolver SourceResolver,
	candRepo CandidateRepository,
	matcher *Matcher,
	svc *Service,
) jobs.HandlerFunc {
	return func(ctx context.Context, rawPayload json.RawMessage, report jobs.ReportProgressFunc) error {
		var payload ImportJobPayload
		if err := json.Unmarshal(rawPayload, &payload); err != nil {
			return jobs.Permanent(fmt.Errorf("invalid import job payload: %w", err))
		}

		now := time.Now().UTC()

		// 1. Progress 10%: Start download
		if report != nil {
			report(10, 100)
		}

		stream, err := resolver.Resolve(ctx, payload.SourceID, payload.FileReference)
		if err != nil {
			failErr := fmt.Sprintf("resolving source file: %v", err)
			_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
			return jobs.Permanent(fmt.Errorf("resolving source file: %w", err))
		}
		defer func() { _ = stream.Close() }()

		// 2. Materialize up to 250 MiB
		tmpFile, cleanup, err := extract.Materialize(ctx, stream)
		if err != nil {
			failErr := fmt.Sprintf("spooling source file: %v", err)
			_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
			if errors.Is(err, extract.ErrOversized) {
				return jobs.Permanent(err)
			}
			return err
		}
		defer cleanup()

		// 3. Progress 40%: Detect format & extract
		if report != nil {
			report(40, 100)
		}

		format, err := extract.DetectFormat(tmpFile)
		if err != nil {
			failErr := fmt.Sprintf("detecting format: %v", err)
			_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
			return jobs.Permanent(err)
		}
		if format == extract.FormatUnknown {
			failErr := "unknown or unsupported file format"
			_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
			return jobs.Permanent(errors.New(failErr))
		}

		meta, err := extract.Extract(ctx, format, tmpFile)
		if err != nil {
			failErr := fmt.Sprintf("extracting metadata: %v", err)
			_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
			if errors.Is(err, extract.ErrNoTitle) || errors.Is(err, extract.ErrMalformed) ||
				errors.Is(err, extract.ErrOversized) || errors.Is(err, extract.ErrTooManyEntries) {
				return jobs.Permanent(err)
			}
			return err
		}

		// 4. Progress 70%: Match candidates
		if report != nil {
			report(70, 100)
		}

		matches, autoAccept, err := matcher.Match(ctx, meta)
		if err != nil {
			failErr := fmt.Sprintf("matching records: %v", err)
			_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
			return err
		}

		// 5. Progress 90%: Persist or auto-import
		if report != nil {
			report(90, 100)
		}

		if autoAccept && svc != nil && len(matches) == 1 && matches[0].EditionID != nil {
			if err := svc.AutoImport(ctx, payload.CandidateID, payload.SourceID, *matches[0].EditionID, payload.FileReference, format, now); err != nil {
				failErr := fmt.Sprintf("auto-importing candidate: %v", err)
				_ = candRepo.UpdateFailed(ctx, payload.CandidateID, failErr, now)
				return err
			}
		} else {
			metaJSON, err := json.Marshal(meta)
			if err != nil {
				return fmt.Errorf("marshalling extracted metadata: %w", err)
			}
			matchJSON, err := json.Marshal(matches)
			if err != nil {
				return fmt.Errorf("marshalling match candidates: %w", err)
			}

			if err := candRepo.UpdateExtractedAndMatches(ctx, payload.CandidateID, postgres.ImportCandidateStatusPending, metaJSON, matchJSON, now); err != nil {
				return fmt.Errorf("updating candidate: %w", err)
			}
		}

		// 6. Progress 100%: Completed
		if report != nil {
			report(100, 100)
		}
		return nil
	}
}
