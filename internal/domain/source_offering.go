package domain

import "time"

// SourceOfferingKey is the (Source, Edition, Format) tuple
// that uniquely identifies a SourceOffering — the same Source
// offering the same Edition in two Formats is two rows; re-observing the
// same Format updates that row's timestamp, keyed on this tuple.
type SourceOfferingKey struct {
	SourceID  SourceID
	EditionID EditionID
	Format    string
}

// SourceOffering represents a content offering from a Source for a specific
// Edition and FileReference, observed at a timestamp.
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

// UniquenessKey returns the (Source, Edition, Format) tuple that defines
// this offering's natural identity for upsert operations.
func (o *SourceOffering) UniquenessKey() SourceOfferingKey {
	return SourceOfferingKey{SourceID: o.sourceID, EditionID: o.editionID, Format: o.fileReference.Format}
}
