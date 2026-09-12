package domain

// maxTitleLength defines the maximum allowed length shared by Work's title and subtitle.
const maxTitleLength = 500

// Work represents a conceptual intellectual work with an internal ID and
// optional external references. A Work with zero Editions and zero external
// references is a valid state; Editions reference their parent Work by ID,
// rather than being embedded on the Work. MergedInto and Contains are managed
// exclusively by domain services, never set directly in constructors.
type Work struct {
	id                 WorkID
	title              string
	subtitle           string
	authors            []AuthorID
	subjects           []Subject
	originalLanguage   *Language
	externalReferences []ExternalReference
	mergedInto         *WorkID
	contains           []WorkID
}

// NewWork validates title (required) and subtitle (optional — an empty
// subtitle skips validation entirely, a present one is validated the
// same as title) against validate.go's bounded-text check.
func NewWork(
	id WorkID,
	title string,
	subtitle string,
	authors []AuthorID,
	subjects []Subject,
	originalLanguage *Language,
	externalReferences []ExternalReference,
) (*Work, error) {
	if err := ValidateBoundedText("title", title, maxTitleLength); err != nil {
		return nil, err
	}
	if subtitle != "" {
		if err := ValidateBoundedText("subtitle", subtitle, maxTitleLength); err != nil {
			return nil, err
		}
	}
	return &Work{
		id:                 id,
		title:              title,
		subtitle:           subtitle,
		authors:            authors,
		subjects:           subjects,
		originalLanguage:   originalLanguage,
		externalReferences: externalReferences,
	}, nil
}

func (w *Work) ID() WorkID { return w.id }

func (w *Work) Title() string { return w.title }

func (w *Work) Subtitle() string { return w.subtitle }

func (w *Work) Authors() []AuthorID { return w.authors }

func (w *Work) Subjects() []Subject { return w.subjects }

func (w *Work) OriginalLanguage() *Language { return w.originalLanguage }

func (w *Work) ExternalReferences() []ExternalReference { return w.externalReferences }

func (w *Work) MergedInto() *WorkID { return w.mergedInto }

func (w *Work) Contains() []WorkID { return w.contains }
