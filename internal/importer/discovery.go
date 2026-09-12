package importer

import (
	"context"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// DiscoverResult holds summary metrics for an import discovery run.
type DiscoverResult struct {
	DiscoveredCount int      `json:"discoveredCount"`
	SkippedCount    int      `json:"skippedCount"`
	JobIDs          []string `json:"jobIds"`
}

// SourceChecker verifies source existence and capabilities.
type SourceChecker interface {
	GetSource(ctx context.Context, sourceID string) (*domain.Source, error)
}

// SourceLister lists file candidates from a source provider.
type SourceLister interface {
	ListSource(ctx context.Context, sourceID string, cursor string, limit int) (sources.CandidatePage, error)
}

// JobEnqueuer enqueues background import jobs.
type JobEnqueuer interface {
	EnqueueImportJob(ctx context.Context, payload ImportJobPayload) (string, error)
}

// DiscoveryCoordinator discovers source files, deduplicates against candidates/offerings, and enqueues jobs.
type DiscoveryCoordinator struct {
	checker  SourceChecker
	lister   SourceLister
	candRepo CandidateRepository
	enqueuer JobEnqueuer
	ids      domain.IDGenerator
}

// NewDiscoveryCoordinator creates a new DiscoveryCoordinator.
func NewDiscoveryCoordinator(
	checker SourceChecker,
	lister SourceLister,
	candRepo CandidateRepository,
	enqueuer JobEnqueuer,
	ids domain.IDGenerator,
) *DiscoveryCoordinator {
	return &DiscoveryCoordinator{
		checker:  checker,
		lister:   lister,
		candRepo: candRepo,
		enqueuer: enqueuer,
		ids:      ids,
	}
}

// Discover enumerates a source's files, checks deduplication, and creates queued candidate records and jobs.
func (c *DiscoveryCoordinator) Discover(ctx context.Context, sourceID string, now time.Time) (DiscoverResult, error) {
	src, err := c.checker.GetSource(ctx, sourceID)
	if err != nil {
		return DiscoverResult{}, err
	}
	if !src.Capabilities().CanList {
		return DiscoverResult{}, &domain.Error{Category: domain.InvalidInput, Message: "source does not support listing"}
	}

	var discoveredCount, skippedCount int
	var jobIDs []string
	cursor := ""

	for {
		if ctx.Err() != nil {
			return DiscoverResult{}, ctx.Err()
		}

		page, err := c.lister.ListSource(ctx, sourceID, cursor, sources.MaxLimit)
		if err != nil {
			return DiscoverResult{}, fmt.Errorf("listing items from source: %w", err)
		}

		for _, item := range page.Items {
			exists, err := c.candRepo.ExistsBySourceAndFileRefID(ctx, sourceID, item.FileReference.ReferenceID)
			if err != nil {
				return DiscoverResult{}, fmt.Errorf("checking candidate existence: %w", err)
			}
			if exists {
				skippedCount++
				continue
			}

			candID := c.ids.NewID()
			payload := ImportJobPayload{
				CandidateID:   candID,
				SourceID:      sourceID,
				FileReference: item.FileReference,
			}

			jobID, err := c.enqueuer.EnqueueImportJob(ctx, payload)
			if err != nil {
				return DiscoverResult{}, fmt.Errorf("enqueuing import job: %w", err)
			}

			rec := postgres.ImportCandidateRecord{
				ID:            candID,
				SourceID:      sourceID,
				FileReference: item.FileReference,
				Status:        postgres.ImportCandidateStatusQueued,
				JobID:         &jobID,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := c.candRepo.Create(ctx, rec); err != nil {
				return DiscoverResult{}, fmt.Errorf("persisting candidate record: %w", err)
			}

			discoveredCount++
			jobIDs = append(jobIDs, jobID)
		}

		if page.NextCursor == nil || *page.NextCursor == "" {
			break
		}
		cursor = *page.NextCursor
	}

	return DiscoverResult{
		DiscoveredCount: discoveredCount,
		SkippedCount:    skippedCount,
		JobIDs:          jobIDs,
	}, nil
}
