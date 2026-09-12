package domain

// maxSourceLabelLength defines the maximum allowed length for a source label.
const maxSourceLabelLength = 100

// SourceCapabilities represents what operations a Source supports.
// The domain never infers capabilities from source type; a Source declares them explicitly.
type SourceCapabilities struct {
	CanList     bool
	CanSearch   bool
	CanDownload bool
}

// Source represents a content provider with internal ID, user-assigned label,
// and declared capabilities. Kind is an optional, coarse display tag.
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
