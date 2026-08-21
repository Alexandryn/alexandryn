package domain

import "time"

// SourceOfferingKey is the (Source, Edition, Format) tuple
// domain-source.md FR-2 makes SourceOffering unique by — the same Source
// offering the same Edition in two Formats is two rows; re-observing the
// same Format updates that row's timestamp, keyed on this tuple, not on
// SourceOfferingID.
type SourceOfferingKey struct {
	SourceID  SourceID
	EditionID EditionID
	Format    string
}

// SourceOffering (FR-2) — "this Source claims to offer this Edition, as
// this FileReference, observed at this timestamp." Never a boolean; every
// read comes with its observation timestamp (FR-3).
type SourceOffering struct {
	id            SourceOfferingID
	sourceID      SourceID
	editionID     EditionID
	fileReference FileReference
	observedAt    time.Time
}

func NewSourceOffering(id SourceOfferingID, sourceID SourceID, editionID EditionID, fileReference FileReference, observedAt time.Time) *SourceOffering {
	return &SourceOffering{
		id:            id,
		sourceID:      sourceID,
		editionID:     editionID,
		fileReference: fileReference,
		observedAt:    observedAt,
	}
}

func (o *SourceOffering) ID() SourceOfferingID { return o.id }

func (o *SourceOffering) SourceID() SourceID { return o.sourceID }

func (o *SourceOffering) EditionID() EditionID { return o.editionID }

func (o *SourceOffering) FileReference() FileReference { return o.fileReference }

func (o *SourceOffering) ObservedAt() time.Time { return o.observedAt }

// UniquenessKey returns the (Source, Edition, Format) tuple FR-2 defines
// as this offering's real identity — a repository implementation keys
// its upsert on this, not on SourceOfferingID.
func (o *SourceOffering) UniquenessKey() SourceOfferingKey {
	return SourceOfferingKey{SourceID: o.sourceID, EditionID: o.editionID, Format: o.fileReference.Format}
}
