package domain

// maxSubjectLength is a reasoned placeholder (no spec gives a number for
// this field), sized for a genre/topic tag, not a description — same
// unmeasured-placeholder status as this project's other numeric bounds
// until real data exists to size it against.
const maxSubjectLength = 100

// Subject is a bounded, printable text value (domain-bibliographic.md
// FR-5) — a value type with no identity of its own, compared by value.
type Subject struct {
	value string
}

// NewSubject validates value against validate.go's bounded-text check.
func NewSubject(value string) (Subject, error) {
	if err := ValidateBoundedText("subject", value, maxSubjectLength); err != nil {
		return Subject{}, err
	}
	return Subject{value: value}, nil
}

func (s Subject) String() string { return s.value }
