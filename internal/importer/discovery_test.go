package importer_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

type fakeSourceLister struct {
	items []sources.SourceCandidate
	err   error
}

func (l *fakeSourceLister) ListSource(ctx context.Context, sourceID string, cursor string, limit int) (sources.CandidatePage, error) {
	if l.err != nil {
		return sources.CandidatePage{}, l.err
	}
	return sources.CandidatePage{
		Items:      l.items,
		NextCursor: nil,
	}, nil
}

type fakeJobEnqueuer struct {
	enqueued []importer.ImportJobPayload
}

func (q *fakeJobEnqueuer) EnqueueImportJob(ctx context.Context, payload importer.ImportJobPayload) (string, error) {
	q.enqueued = append(q.enqueued, payload)
	return "job-" + payload.CandidateID, nil
}

type fakeSourceChecker struct {
	source *domain.Source
	err    error
}

func (c *fakeSourceChecker) GetSource(ctx context.Context, sourceID string) (*domain.Source, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.source, nil
}

func TestDiscover_DiscoversNewAndSkipsExisting(t *testing.T) {
	ctx := context.Background()

	ref1, _ := domain.NewFileReference("ref-1", "epub", nil)
	ref2, _ := domain.NewFileReference("ref-2", "pdf", nil)

	src, _ := domain.NewSource("src-1", "My Library", domain.SourceCapabilities{CanList: true, CanSearch: true, CanDownload: true}, "local-folder")
	checker := &fakeSourceChecker{source: src}

	lister := &fakeSourceLister{
		items: []sources.SourceCandidate{
			{Title: "Book 1", FileReference: ref1},
			{Title: "Book 2", FileReference: ref2},
		},
	}

	candRepo := &fakeCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"existing-cand": {
				ID:            "existing-cand",
				SourceID:      "src-1",
				FileReference: ref1, // ref1 already exists
				Status:        postgres.ImportCandidateStatusPending,
			},
		},
	}

	enqueuer := &fakeJobEnqueuer{}
	idGen := idgen.New()

	coordinator := importer.NewDiscoveryCoordinator(checker, lister, candRepo, enqueuer, idGen)

	res, err := coordinator.Discover(ctx, "src-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// ref1 skipped (already in candRepo), ref2 discovered
	if res.DiscoveredCount != 1 {
		t.Errorf("DiscoveredCount = %d, want 1", res.DiscoveredCount)
	}
	if res.SkippedCount != 1 {
		t.Errorf("SkippedCount = %d, want 1", res.SkippedCount)
	}
	if len(enqueuer.enqueued) != 1 || enqueuer.enqueued[0].FileReference.ReferenceID != "ref-2" {
		t.Errorf("enqueued = %+v, want ref-2", enqueuer.enqueued)
	}
}

func TestDiscover_NonListableSourceRejection(t *testing.T) {
	ctx := context.Background()
	src, _ := domain.NewSource("src-nolist", "No List", domain.SourceCapabilities{CanList: false, CanSearch: true, CanDownload: false}, "openlibrary")
	checker := &fakeSourceChecker{source: src}

	coordinator := importer.NewDiscoveryCoordinator(checker, nil, nil, nil, nil)
	_, err := coordinator.Discover(ctx, "src-nolist", time.Now().UTC())
	if err == nil || domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("err = %v, want InvalidInput", err)
	}
}
