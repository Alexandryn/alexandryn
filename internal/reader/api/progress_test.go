package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/reader/api"
)

type memProgressStore struct {
	byWork map[domain.WorkID]*domain.ReadingProgress
}

func newStore() *memProgressStore {
	return &memProgressStore{byWork: map[domain.WorkID]*domain.ReadingProgress{}}
}
func (s *memProgressStore) FindByWorkForUpdate(_ context.Context, w domain.WorkID) (*domain.ReadingProgress, error) {
	if p, ok := s.byWork[w]; ok {
		return p, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "not found"}
}
func (s *memProgressStore) Save(_ context.Context, p *domain.ReadingProgress) error {
	s.byWork[p.WorkID()] = p
	return nil
}

type inlineTx struct{}

func (inlineTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type seqIDs struct{ n int }

func (s *seqIDs) NewID() string { s.n++; return "progress-" + string(rune('0'+s.n)) }

func mustPct(t *testing.T, v float64) domain.Percentage {
	t.Helper()
	p, err := domain.NewPercentage(v)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func report(t *testing.T, pct float64, epoch int64, dev string) domain.ProgressReport {
	return domain.ProgressReport{
		WorkID: "work-1", Percentage: mustPct(t, pct), ObservedEpoch: epoch,
		DeviceID: domain.DeviceID(dev), ReportedAt: time.Unix(epoch*100, 0),
	}
}

func TestReportProgress_FirstReportBecomesCanonical(t *testing.T) {
	store := newStore()
	got, outcome, err := api.ReportProgress(context.Background(), inlineTx{}, store, &seqIDs{}, report(t, 0.3, 0, "dev-a"), false)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != domain.ReconcileAdvanced || got.Percentage() != mustPct(t, 0.3) || got.Epoch() != 0 {
		t.Fatalf("first report: outcome=%q pct=%v epoch=%d", outcome, got.Percentage(), got.Epoch())
	}
}

func TestReportProgress_BehindIsRejectedNoWrite(t *testing.T) {
	store := newStore()
	_, _, _ = api.ReportProgress(context.Background(), inlineTx{}, store, &seqIDs{}, report(t, 0.6, 0, "dev-a"), false)

	got, outcome, err := api.ReportProgress(context.Background(), inlineTx{}, store, &seqIDs{}, report(t, 0.2, 0, "dev-b"), false)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != domain.ReconcileRejected {
		t.Fatalf("outcome = %q, want rejected", outcome)
	}
	if got.Percentage() != mustPct(t, 0.6) || got.DeviceID() != "dev-a" {
		t.Fatalf("canonical moved on a rejected report: %v / %v", got.Percentage(), got.DeviceID())
	}
}

func TestReportProgress_OverrideBumpsEpoch(t *testing.T) {
	store := newStore()
	_, _, _ = api.ReportProgress(context.Background(), inlineTx{}, store, &seqIDs{}, report(t, 0.9, 0, "dev-a"), false)

	got, outcome, err := api.ReportProgress(context.Background(), inlineTx{}, store, &seqIDs{}, report(t, 0.1, 0, "dev-b"), true)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != domain.ReconcileOverridden || got.Epoch() != 1 || got.Percentage() != mustPct(t, 0.1) {
		t.Fatalf("override: outcome=%q epoch=%d pct=%v", outcome, got.Epoch(), got.Percentage())
	}

	// A subsequent old-epoch report is rejected.
	_, outcome2, _ := api.ReportProgress(context.Background(), inlineTx{}, store, &seqIDs{}, report(t, 0.95, 0, "dev-c"), false)
	if outcome2 != domain.ReconcileRejected {
		t.Fatalf("stale report after override: outcome = %q, want rejected", outcome2)
	}
}
