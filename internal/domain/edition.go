package domain

// Edition represents a specific publication of a Work. An Edition cannot be
// constructed without a parent Work: WorkID is a required, non-pointer field,
// ensuring there is no valid instance without a parent work.
// Language is the edition's specific language, independent of Work.OriginalLanguage,
// allowing translations to be modeled accurately. ISBN, publisher, and publication
// year are all optional.
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
