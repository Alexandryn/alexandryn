package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// domain-reading.md FR-1: ReadingProgress MUST attach to Work, not
// Edition or File — WorkID is a required, non-pointer positional
// argument, the same type-level guarantee Edition's own required WorkID
// gives (domain-bibliographic.md FR-8's pattern, reused here).
func TestNewReadingProgress(t *testing.T) {
	pct, _ := domain.NewPercentage(0.5)
	now := time.Now()

	rp := domain.NewReadingProgress("progress-1", "work-1", pct, "device-1", now)

	if rp.WorkID() != "work-1" {
		t.Fatalf("WorkID() = %v, want work-1", rp.WorkID())
	}
	if rp.Percentage() != pct {
		t.Fatalf("Percentage() = %v, want %v", rp.Percentage(), pct)
	}
	if rp.DeviceID() != "device-1" {
		t.Fatalf("DeviceID() = %v, want device-1", rp.DeviceID())
	}
	if !rp.ObservedAt().Equal(now) {
		t.Fatalf("ObservedAt() = %v, want %v", rp.ObservedAt(), now)
	}
	// FR-2's fallback rule: a PrecisePosition is used only when reading
	// the same Edition again — at construction, before any position is
	// attached (P20), there is none.
	if rp.PrecisePosition() != nil {
		t.Fatalf("PrecisePosition() = %v, want nil at construction", rp.PrecisePosition())
	}
}

// FR-2: ProgressReport is a distinct, ephemeral type, never itself
// persisted — it exists only as ReconcileProgress's input (FR-6, not
// built in this plan; domain-reading.md's FR-6/FR-7 are separately
// blocked). Built here as a plain value type since nothing constructs or
// validates it beyond field assignment — there is no domain operation in
// this plan that consumes one yet.
func TestProgressReport_IsAPlainValueType(t *testing.T) {
	pct, _ := domain.NewPercentage(0.75)
	report := domain.ProgressReport{
		WorkID:     "work-1",
		Percentage: pct,
		DeviceID:   "device-2",
		ReportedAt: time.Now(),
	}
	if report.WorkID != "work-1" {
		t.Fatalf("WorkID = %v, want work-1", report.WorkID)
	}
}
