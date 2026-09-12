package domain_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// ReadingProgress attaches to Work, not Edition or File — WorkID is a
// required, non-pointer positional argument.
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
	// At construction, before any position is attached, PrecisePosition is nil.
	if rp.PrecisePosition() != nil {
		t.Fatalf("PrecisePosition() = %v, want nil at construction", rp.PrecisePosition())
	}
}

// ProgressReport is a distinct, ephemeral type, never itself persisted — it
// exists only as ReconcileProgress's input.
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
