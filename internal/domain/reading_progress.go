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
	epoch           int64
	precisePosition *PrecisePosition
	deviceID        DeviceID
	observedAt      time.Time
}

// NewReadingProgress constructs the first canonical value for a Work —
// epoch 0 by construction (domain-reading.md's State transitions: "first
// ProgressReport becomes the canonical ReadingProgress directly, Epoch
// := 0"). A later epoch is only ever reached through OverrideProgress
// (FR-7); ReconcileProgress never raises it.
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

// Epoch is the server-assigned, monotonically non-decreasing generation
// counter reconciliation orders on before percentage (domain-reading.md
// FR-2/FR-6). Bumped only by OverrideProgress (FR-7).
func (r *ReadingProgress) Epoch() int64 { return r.epoch }

func (r *ReadingProgress) PrecisePosition() *PrecisePosition { return r.precisePosition }

func (r *ReadingProgress) DeviceID() DeviceID { return r.deviceID }

func (r *ReadingProgress) ObservedAt() time.Time { return r.observedAt }

// ProgressReport (FR-2) is ephemeral and never persisted — it exists
// only as ReconcileProgress's input (FR-6/FR-7, separately blocked, not
// built in this plan). A plain value type: nothing in this plan
// constructs or validates one beyond field assignment, since nothing
// here consumes it yet.
type ProgressReport struct {
	WorkID     WorkID
	Percentage Percentage
	// ObservedEpoch is the Epoch the reporting device last received from
	// the server for this Work (0 if it has never synced) —
	// ReconcileProgress's ordering key is (ObservedEpoch, Percentage),
	// clamped to the stored Epoch (domain-reading.md FR-6 as amended).
	ObservedEpoch   int64
	PrecisePosition *PrecisePosition
	DeviceID        DeviceID
	ReportedAt      time.Time
}
