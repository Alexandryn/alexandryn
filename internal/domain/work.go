package domain

// maxTitleLength is a reasoned placeholder (no spec gives a number),
// shared by Work's title and subtitle.
const maxTitleLength = 500

// Work (domain-bibliographic.md FR-1/FR-3/FR-6) — internal ID primary,
// external references optional. A Work with zero Editions and zero
// external references is a legal, permanent state (FR-3), not an error
// or a "pending" case; Editions themselves aren't a field here — they
// reference their parent Work by ID (FR-2), never the reverse embedding.
// MergedInto and Contains are empty at construction — both are
// domain-service operations (ADR 0020, this plan's Tier 2), never
// constructor arguments.
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
