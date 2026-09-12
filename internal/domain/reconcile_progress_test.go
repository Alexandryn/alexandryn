package domain_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

func mustPct(t *testing.T, v float64) domain.Percentage {
	t.Helper()
	p, err := domain.NewPercentage(v)
	if err != nil {
		t.Fatalf("NewPercentage(%v): %v", v, err)
	}
	return p
}

// canonicalAt builds a stored ReadingProgress at a given epoch/percentage
// for reconcile tests — RehydrateReadingProgress is the "already stored"
// path and carries epoch.
func canonicalAt(t *testing.T, epoch int64, pct float64, dev domain.DeviceID) *domain.ReadingProgress {
	t.Helper()
	return domain.RehydrateReadingProgress("progress-1", "work-1", mustPct(t, pct), epoch, nil, dev, time.Unix(0, 0))
}

// A further same-epoch percentage advances the canonical value; the new value
// takes the report's percentage, position, device, and reported-at.
func TestReconcileProgress_FurtherSameEpochAdvances(t *testing.T) {
	canonical := canonicalAt(t, 2, 0.40, "device-A")
	report := domain.ProgressReport{
		WorkID:          "work-1",
		Percentage:      mustPct(t, 0.55),
		ObservedEpoch:   2,
		PrecisePosition: &domain.PrecisePosition{EditionID: "edition-1", Value: "epubcfi(/6/4!/4)"},
		DeviceID:        "device-B",
		ReportedAt:      time.Unix(100, 0),
	}

	got := domain.ReconcileProgress(canonical, report)

	if got.Outcome != domain.ReconcileAdvanced {
		t.Fatalf("Outcome = %q, want advanced", got.Outcome)
	}
	if got.Progress.Percentage() != mustPct(t, 0.55) {
		t.Fatalf("Percentage() = %v, want 0.55", got.Progress.Percentage())
	}
	if got.Progress.Epoch() != 2 {
		t.Fatalf("Epoch() = %d, want 2 (unchanged by a normal report)", got.Progress.Epoch())
	}
	if got.Progress.DeviceID() != "device-B" {
		t.Fatalf("DeviceID() = %v, want device-B", got.Progress.DeviceID())
	}
	if got.Progress.ID() != canonical.ID() {
		t.Fatalf("ID() = %v, want the canonical's stable id %v", got.Progress.ID(), canonical.ID())
	}
	if pos := got.Progress.PrecisePosition(); pos == nil || pos.Value != "epubcfi(/6/4!/4)" {
		t.Fatalf("PrecisePosition() = %v, want the report's", pos)
	}
}

// An equal (epoch, percentage) key is Unchanged — a no-op, provenance
// not rewritten (this is what makes the fold deterministic without a tiebreak).
func TestReconcileProgress_EqualKeyIsUnchanged(t *testing.T) {
	canonical := canonicalAt(t, 1, 0.50, "device-A")
	report := domain.ProgressReport{
		WorkID: "work-1", Percentage: mustPct(t, 0.50), ObservedEpoch: 1,
		DeviceID: "device-B", ReportedAt: time.Unix(100, 0),
	}

	got := domain.ReconcileProgress(canonical, report)

	if got.Outcome != domain.ReconcileUnchanged {
		t.Fatalf("Outcome = %q, want unchanged", got.Outcome)
	}
	if got.Progress.DeviceID() != "device-A" {
		t.Fatalf("provenance rewritten: DeviceID() = %v, want the canonical's device-A", got.Progress.DeviceID())
	}
}

// A lower same-epoch percentage is Rejected — the canonical value
// is returned unchanged.
func TestReconcileProgress_BehindIsRejected(t *testing.T) {
	canonical := canonicalAt(t, 1, 0.60, "device-A")
	report := domain.ProgressReport{
		WorkID: "work-1", Percentage: mustPct(t, 0.30), ObservedEpoch: 1,
		DeviceID: "device-B", ReportedAt: time.Unix(100, 0),
	}

	got := domain.ReconcileProgress(canonical, report)

	if got.Outcome != domain.ReconcileRejected {
		t.Fatalf("Outcome = %q, want rejected", got.Outcome)
	}
	if got.Progress.Percentage() != mustPct(t, 0.60) {
		t.Fatalf("canonical moved: Percentage() = %v, want 0.60", got.Progress.Percentage())
	}
}

// A report whose ObservedEpoch is below the stored Epoch (a device
// offline across an override) is Rejected regardless of its percentage.
func TestReconcileProgress_StaleEpochIsRejectedEvenIfFurther(t *testing.T) {
	canonical := canonicalAt(t, 3, 0.20, "device-A")
	report := domain.ProgressReport{
		WorkID: "work-1", Percentage: mustPct(t, 0.99), ObservedEpoch: 2,
		DeviceID: "device-B", ReportedAt: time.Unix(100, 0),
	}

	got := domain.ReconcileProgress(canonical, report)

	if got.Outcome != domain.ReconcileRejected {
		t.Fatalf("Outcome = %q, want rejected (stale epoch beats a further percentage)", got.Outcome)
	}
	if got.Progress.Percentage() != mustPct(t, 0.20) || got.Progress.Epoch() != 3 {
		t.Fatalf("canonical moved: %v @ %d, want 0.20 @ 3", got.Progress.Percentage(), got.Progress.Epoch())
	}
}

// A client cannot advance the epoch by over-reporting ObservedEpoch —
// it is clamped to the stored value, so the report only competes on percentage.
func TestReconcileProgress_OverReportedEpochIsClamped(t *testing.T) {
	canonical := canonicalAt(t, 1, 0.40, "device-A")
	report := domain.ProgressReport{
		WorkID: "work-1", Percentage: mustPct(t, 0.50), ObservedEpoch: 9999,
		DeviceID: "device-B", ReportedAt: time.Unix(100, 0),
	}

	got := domain.ReconcileProgress(canonical, report)

	if got.Outcome != domain.ReconcileAdvanced {
		t.Fatalf("Outcome = %q, want advanced", got.Outcome)
	}
	if got.Progress.Epoch() != 1 {
		t.Fatalf("Epoch() = %d, want 1 — a client must not be able to fast-forward the epoch", got.Progress.Epoch())
	}
}

// OverrideProgress sets the canonical value to the target percentage
// at Epoch+1 unconditionally — even a backward move.
func TestOverrideProgress_BumpsEpochAndSetsPercentageUnconditionally(t *testing.T) {
	canonical := canonicalAt(t, 2, 0.80, "device-A")
	pos := &domain.PrecisePosition{EditionID: "edition-1", Value: "epubcfi(/6/2!/2)"}

	got := domain.OverrideProgress(canonical, mustPct(t, 0.10), pos, "device-B", time.Unix(200, 0))

	if got.Epoch() != 3 {
		t.Fatalf("Epoch() = %d, want 3 (canonical.Epoch + 1)", got.Epoch())
	}
	if got.Percentage() != mustPct(t, 0.10) {
		t.Fatalf("Percentage() = %v, want 0.10 (the target, set unconditionally)", got.Percentage())
	}
	if got.ID() != canonical.ID() {
		t.Fatalf("ID() = %v, want the canonical's stable id", got.ID())
	}
}

// After an override, every device still reporting the old epoch is
// Rejected until it re-syncs — the re-read sticks.
func TestOverrideThenStaleReports_KeepsTheOverride(t *testing.T) {
	canonical := canonicalAt(t, 1, 0.90, "device-A")
	overridden := domain.OverrideProgress(canonical, mustPct(t, 0.15), nil, "device-B", time.Unix(200, 0))

	// Two old-epoch reports, both further along than the override.
	for _, pctVal := range []float64{0.5, 0.95} {
		report := domain.ProgressReport{
			WorkID: "work-1", Percentage: mustPct(t, pctVal), ObservedEpoch: 1,
			DeviceID: "device-C", ReportedAt: time.Unix(300, 0),
		}
		res := domain.ReconcileProgress(overridden, report)
		if res.Outcome != domain.ReconcileRejected {
			t.Fatalf("stale report at %.2f: Outcome = %q, want rejected", pctVal, res.Outcome)
		}
		overridden = res.Progress
	}
	if overridden.Percentage() != mustPct(t, 0.15) || overridden.Epoch() != 2 {
		t.Fatalf("override lost: %v @ %d, want 0.15 @ 2", overridden.Percentage(), overridden.Epoch())
	}
}

// Folding any number of well-formed same-epoch reports
// in any order produces the same canonical value — max over percentage at
// the fixed epoch. Generator emits no overrides.
func TestReconcileProgress_SameEpochFoldIsOrderIndependent(t *testing.T) {
	rng := rand.New(rand.NewSource(1))

	for trial := 0; trial < 200; trial++ {
		const epoch = 4
		startPct := rng.Float64()
		reports := make([]domain.ProgressReport, rng.Intn(6)+2)
		want := startPct
		for i := range reports {
			p := rng.Float64()
			if p > want {
				want = p
			}
			reports[i] = domain.ProgressReport{
				WorkID: "work-1", Percentage: mustPct(t, p), ObservedEpoch: epoch,
				DeviceID: domain.DeviceID("device"), ReportedAt: time.Unix(int64(i), 0),
			}
		}

		fold := func(order []int) *domain.ReadingProgress {
			cur := canonicalAt(t, epoch, startPct, "device-seed")
			for _, idx := range order {
				cur = domain.ReconcileProgress(cur, reports[idx]).Progress
			}
			return cur
		}

		forward := make([]int, len(reports))
		for i := range forward {
			forward[i] = i
		}
		shuffled := append([]int(nil), forward...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })

		a := fold(forward)
		b := fold(shuffled)

		if a.Percentage() != b.Percentage() {
			t.Fatalf("trial %d: order-dependent result %v vs %v", trial, a.Percentage(), b.Percentage())
		}
		if float64(a.Percentage()) != want {
			t.Fatalf("trial %d: folded to %v, want max %v", trial, a.Percentage(), want)
		}
		if a.Epoch() != epoch {
			t.Fatalf("trial %d: epoch drifted to %d", trial, a.Epoch())
		}
	}
}
