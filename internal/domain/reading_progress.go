package domain

import "time"

// PrecisePosition (domain-reading.md's own Non-goals: "this spec treats
// a position as an opaque, edition-scoped value... phase 11 picks the
// concrete representation") — Value's format (CFI, page number, or
// something else) is deliberately undecided here.
type PrecisePosition struct {
	EditionID EditionID
	Value     string
}

// ReadingProgress (FR-1/FR-2) — the canonical, singleton-per-Work record.
// WorkID is a required, non-pointer positional argument, the same
// type-level guarantee Edition's own required WorkID gives
// (domain-bibliographic.md FR-8's pattern, reused here). PrecisePosition
// is nil at construction — attaching one requires verifying it belongs
// to this same Work (ADR 0020's own named example), which is
// AttachPrecisePosition's job (P20), not the constructor's.
type ReadingProgress struct {
	id              ReadingProgressID
	workID          WorkID
	percentage      Percentage
	precisePosition *PrecisePosition
	deviceID        DeviceID
	observedAt      time.Time
}

func NewReadingProgress(id ReadingProgressID, workID WorkID, percentage Percentage, deviceID DeviceID, observedAt time.Time) *ReadingProgress {
	return &ReadingProgress{
		id:         id,
		workID:     workID,
		percentage: percentage,
		deviceID:   deviceID,
		observedAt: observedAt,
	}
}

func (r *ReadingProgress) ID() ReadingProgressID { return r.id }

func (r *ReadingProgress) WorkID() WorkID { return r.workID }

func (r *ReadingProgress) Percentage() Percentage { return r.percentage }

func (r *ReadingProgress) PrecisePosition() *PrecisePosition { return r.precisePosition }

func (r *ReadingProgress) DeviceID() DeviceID { return r.deviceID }

func (r *ReadingProgress) ObservedAt() time.Time { return r.observedAt }

// ProgressReport (FR-2) is ephemeral and never persisted — it exists
// only as ReconcileProgress's input (FR-6/FR-7, separately blocked, not
// built in this plan). A plain value type: nothing in this plan
// constructs or validates one beyond field assignment, since nothing
// here consumes it yet.
type ProgressReport struct {
	WorkID          WorkID
	Percentage      Percentage
	PrecisePosition *PrecisePosition
	DeviceID        DeviceID
	ReportedAt      time.Time
}
