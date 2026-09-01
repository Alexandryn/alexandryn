package domain

import "time"

// ReconcileOutcome tags what ReconcileProgress did with a report
// (domain-reading.md FR-6/FR-7). It is a per-call label; it does not
// affect the fold's final canonical value.
type ReconcileOutcome string

const (
	// ReconcileAdvanced — the report's key was strictly greater; the
	// canonical value moved to it.
	ReconcileAdvanced ReconcileOutcome = "advanced"
	// ReconcileUnchanged — the report's key equalled the canonical's; a
	// no-op, provenance not rewritten.
	ReconcileUnchanged ReconcileOutcome = "unchanged"
	// ReconcileRejected — the report was behind the canonical value, or
	// against a superseded epoch; the canonical value is returned
	// unchanged.
	ReconcileRejected ReconcileOutcome = "rejected"
	// ReconcileOverridden — set by OverrideProgress's result when the
	// transport wraps it in a ReconcileResult for a uniform response.
	ReconcileOverridden ReconcileOutcome = "overridden"
)

// ReconcileResult is ReconcileProgress's return: the next canonical
// ReadingProgress and an outcome tag.
type ReconcileResult struct {
	Progress *ReadingProgress
	Outcome  ReconcileOutcome
}

// ReconcileProgress folds one incoming ProgressReport into the canonical
// singleton ReadingProgress for a Work (domain-reading.md FR-6). The
// rule is lexical max over the total order (epoch, percentage):
//
//   - report.ObservedEpoch below the stored Epoch -> Rejected (the
//     device was offline across an override; it must re-sync).
//   - otherwise the report competes at the stored Epoch (an
//     ObservedEpoch above it is clamped down — only the server assigns
//     Epoch), so the comparison reduces to percentage:
//   - strictly greater -> Advanced (canonical takes the report's
//     percentage, position, device, and reported-at; Epoch unchanged).
//   - equal            -> Unchanged (no mutation, provenance kept).
//   - strictly less    -> Rejected.
//
// canonical MUST be non-nil: the "no ReadingProgress exists yet" case is
// the caller's (State transitions — the first report becomes the
// canonical value directly, at Epoch 0), because only the caller can
// mint the row's id.
//
// Over the set of well-formed same-epoch reports the result is max over
// a fixed multiset under a total order, so folding in any order yields
// the same canonical value — the property multi-device sync (phase 14)
// depends on.
func ReconcileProgress(canonical *ReadingProgress, report ProgressReport) ReconcileResult {
	if report.ObservedEpoch < canonical.epoch {
		return ReconcileResult{Progress: canonical, Outcome: ReconcileRejected}
	}

	switch {
	case report.Percentage > canonical.percentage:
		next := &ReadingProgress{
			id:              canonical.id,
			workID:          canonical.workID,
			percentage:      report.Percentage,
			epoch:           canonical.epoch,
			precisePosition: report.PrecisePosition,
			deviceID:        report.DeviceID,
			observedAt:      report.ReportedAt,
		}
		return ReconcileResult{Progress: next, Outcome: ReconcileAdvanced}
	case report.Percentage == canonical.percentage:
		return ReconcileResult{Progress: canonical, Outcome: ReconcileUnchanged}
	default:
		return ReconcileResult{Progress: canonical, Outcome: ReconcileRejected}
	}
}

// FirstProgress builds the canonical ReadingProgress from the very first
// ProgressReport for a Work — Epoch 0, the report's values taken
// directly (domain-reading.md State transitions: "first ProgressReport
// becomes the canonical ReadingProgress directly", the report's
// ObservedEpoch ignored because there was nothing to observe). The
// caller mints the id.
func FirstProgress(id ReadingProgressID, report ProgressReport) *ReadingProgress {
	return &ReadingProgress{
		id:              id,
		workID:          report.WorkID,
		percentage:      report.Percentage,
		epoch:           0,
		precisePosition: report.PrecisePosition,
		deviceID:        report.DeviceID,
		observedAt:      report.ReportedAt,
	}
}

// OverrideProgress expresses a deliberate backward move (a real re-read,
// domain-reading.md FR-7). It produces a new canonical value at
// canonical.Epoch + 1 with the target percentage, unconditionally — the
// bumped epoch makes the result strictly greater than the old value
// under ReconcileProgress's order, so the override always wins and every
// device still on the old epoch is Rejected until it re-syncs.
//
// This is a deliberate, server-serialised act — it is explicitly outside
// ReconcileProgress's commutativity guarantee, which covers automatic
// background reports only. canonical MUST be non-nil.
func OverrideProgress(canonical *ReadingProgress, target Percentage, position *PrecisePosition, deviceID DeviceID, at time.Time) *ReadingProgress {
	return &ReadingProgress{
		id:              canonical.id,
		workID:          canonical.workID,
		percentage:      target,
		epoch:           canonical.epoch + 1,
		precisePosition: position,
		deviceID:        deviceID,
		observedAt:      at,
	}
}
