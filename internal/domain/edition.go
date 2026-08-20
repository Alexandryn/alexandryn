package domain

// Edition (domain-bibliographic.md FR-2/FR-8/FR-10) — cannot be
// constructed without a parent Work: WorkID is a required, non-pointer
// field, so there is no Go value of this type with "no parent," the same
// property FR-8 requires ("literally unrepresentable... not a nil-check
// convention"). Language is Edition's own field, independent of
// Work.OriginalLanguage — the field that actually varies per translation
// (FR-10). ISBN, publisher, publication year are all optional.
type Edition struct {
	id                 EditionID
	workID             WorkID
	language           Language
	isbn               *string
	publisher          string
	publicationYear    *int
	externalReferences []ExternalReference
}

// NewEdition validates isbn (when present, via validate.go's checksum
// check) and publisher (when present — an empty publisher skips
// validation, same optional-field pattern as Work's subtitle).
func NewEdition(
	id EditionID,
	workID WorkID,
	language Language,
	isbn *string,
	publisher string,
	publicationYear *int,
	externalReferences []ExternalReference,
) (*Edition, error) {
	if isbn != nil {
		if err := ValidateISBN(*isbn); err != nil {
			return nil, err
		}
	}
	if publisher != "" {
		if err := ValidateBoundedText("publisher", publisher, maxTitleLength); err != nil {
			return nil, err
		}
	}
	return &Edition{
		id:                 id,
		workID:             workID,
		language:           language,
		isbn:               isbn,
		publisher:          publisher,
		publicationYear:    publicationYear,
		externalReferences: externalReferences,
	}, nil
}

func (e *Edition) ID() EditionID { return e.id }

func (e *Edition) WorkID() WorkID { return e.workID }

func (e *Edition) Language() Language { return e.language }

func (e *Edition) ISBN() *string { return e.isbn }

func (e *Edition) Publisher() string { return e.publisher }

func (e *Edition) PublicationYear() *int { return e.publicationYear }

func (e *Edition) ExternalReferences() []ExternalReference { return e.externalReferences }
