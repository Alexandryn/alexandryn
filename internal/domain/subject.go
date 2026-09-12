package domain

// maxSubjectLength defines the maximum allowed length for a subject name.
const maxSubjectLength = 100

// Subject is a bounded, printable text value type compared by value.
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
