package domain

import "time"

// PrecisePosition treats a position as an opaque, edition-scoped value —
// Value's format (CFI, page number, or other format) is determined by the reader implementation.
type PrecisePosition struct {
	EditionID EditionID
	Value     string
}

// ReadingProgress is the canonical, singleton-per-Work reading progress record.
// WorkID is a required, non-pointer positional argument. PrecisePosition
// is nil at construction — attaching one requires verifying it belongs
// to this same Work via ReadingProgressService.
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
// epoch 0 by construction. A later epoch is only ever reached through
// OverrideProgress; ReconcileProgress never raises it.
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
// counter reconciliation orders on before percentage. Bumped only by OverrideProgress.
func (r *ReadingProgress) Epoch() int64 { return r.epoch }

func (r *ReadingProgress) PrecisePosition() *PrecisePosition { return r.precisePosition }

func (r *ReadingProgress) DeviceID() DeviceID { return r.deviceID }

func (r *ReadingProgress) ObservedAt() time.Time { return r.observedAt }

// ProgressReport is ephemeral and never persisted — it exists
// only as ReconcileProgress's input.
type ProgressReport struct {
	WorkID     WorkID
	Percentage Percentage
	// ObservedEpoch is the Epoch the reporting device last received from
	// the server for this Work (0 if it has never synced) —
	// ReconcileProgress's ordering key is (ObservedEpoch, Percentage),
	// clamped to the stored Epoch.
	ObservedEpoch   int64
	PrecisePosition *PrecisePosition
	DeviceID        DeviceID
	ReportedAt      time.Time
}
