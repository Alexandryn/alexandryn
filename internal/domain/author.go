package domain

// maxAuthorNameLength is a reasoned placeholder (no spec gives a number),
// sized to accommodate a full name with titles/suffixes.
const maxAuthorNameLength = 200

// Author (domain-bibliographic.md FR-7) supports the same identity
// pattern as Work (FR-1): internal ID primary, optional external
// references, never required. MergedInto is nil at construction —
// recording a merge is a domain-service operation (ADR 0020, this plan's
// Tier 2), not something a constructor decides.
type Author struct {
	id                 AuthorID
	name               string
	externalReferences []ExternalReference
	mergedInto         *AuthorID
}

// NewAuthor validates name against validate.go's bounded-text check.
// externalReferences may be nil or empty — zero external references is a
// fully legal, permanent state (FR-3's reasoning, restated for Author).
func NewAuthor(id AuthorID, name string, externalReferences []ExternalReference) (*Author, error) {
	if err := ValidateBoundedText("name", name, maxAuthorNameLength); err != nil {
		return nil, err
	}
	return &Author{
		id:                 id,
		name:               name,
		externalReferences: externalReferences,
	}, nil
}

func (a *Author) ID() AuthorID { return a.id }

func (a *Author) Name() string { return a.name }

func (a *Author) ExternalReferences() []ExternalReference { return a.externalReferences }

func (a *Author) MergedInto() *AuthorID { return a.mergedInto }
