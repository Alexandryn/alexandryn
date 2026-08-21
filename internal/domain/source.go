package domain

// maxSourceLabelLength is a reasoned placeholder (no spec gives a
// number), matching this project's other short-label bounds.
const maxSourceLabelLength = 100

// SourceCapabilities (domain-source.md FR-1) is a fixed, small,
// extensible set — the domain never infers what a Source can do from its
// type, a Source declares it.
type SourceCapabilities struct {
	CanList     bool
	CanSearch   bool
	CanDownload bool
}

// Source (FR-1) — internal ID, user-assigned label, declared
// capabilities. Kind is an optional, coarse display tag (an icon, a
// grouping label) — a category, not protocol configuration; it implies
// nothing about capabilities and phase 08 owns what values exist, so
// this domain doesn't validate it against a closed vocabulary.
type Source struct {
	id           SourceID
	label        string
	capabilities SourceCapabilities
	kind         string
}

// NewSource validates label against validate.go's bounded-text check.
// kind may be empty — optional, for display purposes only.
func NewSource(id SourceID, label string, capabilities SourceCapabilities, kind string) (*Source, error) {
	if err := ValidateBoundedText("label", label, maxSourceLabelLength); err != nil {
		return nil, err
	}
	return &Source{id: id, label: label, capabilities: capabilities, kind: kind}, nil
}

func (s *Source) ID() SourceID { return s.id }

func (s *Source) Label() string { return s.label }

func (s *Source) Capabilities() SourceCapabilities { return s.capabilities }

func (s *Source) Kind() string { return s.kind }
