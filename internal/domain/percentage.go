package domain

// ReadingStatus is computed from Percentage, never stored separately.
type ReadingStatus string

const (
	NotStarted ReadingStatus = "NotStarted"
	InProgress ReadingStatus = "InProgress"
	Finished   ReadingStatus = "Finished"
)

// Percentage is the canonical, edition-independent, always-meaningful
// primary progress value, constrained to [0.0, 1.0].
type Percentage float64

func NewPercentage(value float64) (Percentage, error) {
	if value < 0.0 || value > 1.0 {
		return 0, &Error{Category: InvalidInput, Message: "percentage must be within [0.0, 1.0]"}
	}
	return Percentage(value), nil
}

// Status computes the reading status: 0.0 -> NotStarted, 1.0 ->
// Finished, otherwise InProgress.
func (p Percentage) Status() ReadingStatus {
	switch p {
	case 0.0:
		return NotStarted
	case 1.0:
		return Finished
	default:
		return InProgress
	}
}
