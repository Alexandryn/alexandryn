//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// failAfterNDeletes wraps a real domain.SourceOfferingRepository and lets
// the first n Delete calls through to the real implementation (a genuine
// write against real Postgres, inside the transaction InTx opened), then
// fails every Delete call after that with a synthetic error — the same
// "deliberately failing after a real write" idiom
// transactor_integration_test.go already uses for InTx's own rollback
// proofs, applied here through SourceRemovalService's real cascade
// instead of a hand-rolled closure, since nothing in this schema
// naturally rejects a second DELETE at the constraint level (Delete on a
// nonexistent or already-processed row is not itself an error).
type failAfterNDeletes struct {
	real domain.SourceOfferingRepository
	n    int
	seen int
}

var errDeliberateFailure = errors.New("deliberate failure: simulated mid-cascade error")

func (f *failAfterNDeletes) FindByID(ctx context.Context, id domain.SourceOfferingID) (*domain.SourceOffering, error) {
	return f.real.FindByID(ctx, id)
}

func (f *failAfterNDeletes) FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.SourceOffering, error) {
	return f.real.FindByEdition(ctx, editionID)
}

func (f *failAfterNDeletes) FindBySource(ctx context.Context, sourceID domain.SourceID) ([]*domain.SourceOffering, error) {
	return f.real.FindBySource(ctx, sourceID)
}

func (f *failAfterNDeletes) Save(ctx context.Context, o *domain.SourceOffering) error {
	return f.real.Save(ctx, o)
}

func (f *failAfterNDeletes) Delete(ctx context.Context, id domain.SourceOfferingID) error {
	f.seen++
	if f.seen > f.n {
		return errDeliberateFailure
	}
	return f.real.Delete(ctx, id)
}

var _ domain.SourceOfferingRepository = (*failAfterNDeletes)(nil)

// Verifies that a multi-step operation's transaction rolls back completely
// on a mid-operation failure. SourceRemovalService's cascade — deleting
// every SourceOffering referencing a Source, then the Source itself — spans
// two repositories inside one Transactor.InTx call.
// This proves it against real Postgres: the first of two SourceOfferings is
// deleted inside the transaction, the second delete fails, and everything —
// including the already-executed first delete — must be exactly as it was
// before Remove was called: both SourceOfferings and the Source itself still
// present, preventing partial cascades. Verified through a second, independent
// connection.
func TestSourceRemovalService_Remove_RollsBackWholeCascadeOnMidOperationFailure(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")

	sourceRepo := postgres.NewSourceRepository(pool)
	realOfferingRepo := postgres.NewSourceOfferingRepository(pool)

	ref1, err := domain.NewFileReference("ref-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference ref1: %v", err)
	}
	ref2, err := domain.NewFileReference("ref-2", "pdf", nil)
	if err != nil {
		t.Fatalf("NewFileReference ref2: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	offering1 := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref1, now)
	offering2 := domain.NewSourceOffering("offering-2", "source-1", "edition-1", ref2, now)
	if err := realOfferingRepo.Save(ctx, offering1); err != nil {
		t.Fatalf("seed offering1: %v", err)
	}
	if err := realOfferingRepo.Save(ctx, offering2); err != nil {
		t.Fatalf("seed offering2: %v", err)
	}

	failingOfferingRepo := &failAfterNDeletes{real: realOfferingRepo, n: 1}
	transactor := postgres.NewTransactor(pool)
	svc := domain.NewSourceRemovalService(sourceRepo, failingOfferingRepo, transactor)

	_, _, err = svc.Remove(ctx, "source-1", time.Now())
	if !errors.Is(err, errDeliberateFailure) {
		t.Fatalf("Remove error = %v, want the deliberate sentinel to propagate", err)
	}

	// Verify through a second, independent connection — not the pool the
	// operation itself used, and not a reused tx handle — that nothing
	// from the cascade survived: not the Source, not either
	// SourceOffering, including the one whose Delete really executed
	// before the second one failed.
	secondPool, err := postgres.NewPool(context.Background(), os.Getenv("TEST_DATABASE_URL"), 5)
	if err != nil {
		t.Fatalf("NewPool (second connection): %v", err)
	}
	defer secondPool.Close()

	secondSourceRepo := postgres.NewSourceRepository(secondPool)
	if _, err := secondSourceRepo.FindByID(context.Background(), "source-1"); err != nil {
		t.Fatalf("FindByID(source-1) after rolled-back Remove: %v, want it still found (the delete must have rolled back)", err)
	}

	secondOfferingRepo := postgres.NewSourceOfferingRepository(secondPool)
	offerings, err := secondOfferingRepo.FindBySource(context.Background(), "source-1")
	if err != nil {
		t.Fatalf("FindBySource via second connection: %v", err)
	}
	if len(offerings) != 2 {
		t.Fatalf("FindBySource via second connection = %d offerings, want 2 (the already-executed first delete must have rolled back along with everything else)", len(offerings))
	}
}

// The success path, for contrast: when nothing fails, the cascade
// really does remove every SourceOffering and the Source itself.
func TestSourceRemovalService_Remove_RemovesWholeCascadeOnSuccess(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()

	mustExecPool(t, pool, "INSERT INTO works (id, title) VALUES ('work-1', 'Title')")
	mustExecPool(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('edition-1', 'work-1', 'en', '')")
	mustExecPool(t, pool, "INSERT INTO sources (id, label, can_list, can_search, can_download, kind) VALUES ('source-1', 'Src', true, true, true, '')")

	sourceRepo := postgres.NewSourceRepository(pool)
	offeringRepo := postgres.NewSourceOfferingRepository(pool)

	ref, err := domain.NewFileReference("ref-1", "epub", nil)
	if err != nil {
		t.Fatalf("NewFileReference: %v", err)
	}
	offering := domain.NewSourceOffering("offering-1", "source-1", "edition-1", ref, time.Now().UTC().Truncate(time.Microsecond))
	if err := offeringRepo.Save(ctx, offering); err != nil {
		t.Fatalf("seed offering: %v", err)
	}

	transactor := postgres.NewTransactor(pool)
	svc := domain.NewSourceRemovalService(sourceRepo, offeringRepo, transactor)

	sourceEvent, offeringEvents, err := svc.Remove(ctx, "source-1", time.Now())
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(offeringEvents) != 1 {
		t.Fatalf("offeringEvents = %v, want 1", offeringEvents)
	}
	if sourceEvent.AggregateID() != "source-1" {
		t.Fatalf("sourceEvent.AggregateID() = %v, want source-1", sourceEvent.AggregateID())
	}

	if _, err := sourceRepo.FindByID(ctx, "source-1"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Source category after successful Remove = %v, want NotFound", domain.CategoryOf(err))
	}
	offerings, err := offeringRepo.FindBySource(ctx, "source-1")
	if err != nil {
		t.Fatalf("FindBySource: %v", err)
	}
	if len(offerings) != 0 {
		t.Fatalf("FindBySource after successful Remove = %v, want empty", offerings)
	}
}
