package domain

// maxAuthorNameLength is a reasoned placeholder (no spec gives a number),
// sized to accommodate a full name with titles/suffixes.
const maxAuthorNameLength = 200

// Author represents a person or group responsible for creating works:
// internal ID primary, optional external references, never required.
// MergedInto is nil at construction — recording a merge is a
// domain-service operation.
type Author struct {
	id                 AuthorID
	name               string
	externalReferences []ExternalReference
	mergedInto         *AuthorID
}

// NewAuthor validates name against bounded-text checks.
// externalReferences may be nil or empty; zero external references is valid.
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
