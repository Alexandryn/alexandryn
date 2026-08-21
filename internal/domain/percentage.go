package domain

// ReadingStatus (FR-8) is computed from Percentage, never stored
// separately — the same computed-not-duplicated pattern
// domain-library.md FR-2 already uses for "in library."
type ReadingStatus string

const (
	NotStarted ReadingStatus = "NotStarted"
	InProgress ReadingStatus = "InProgress"
	Finished   ReadingStatus = "Finished"
)

// Percentage (domain-reading.md's own risk table: "progress never
// exceeds its bounds") is the canonical, edition-independent, always-
// meaningful primary progress value, constrained to [0.0, 1.0].
type Percentage float64

func NewPercentage(value float64) (Percentage, error) {
	if value < 0.0 || value > 1.0 {
		return 0, &Error{Category: InvalidInput, Message: "percentage must be within [0.0, 1.0]"}
	}
	return Percentage(value), nil
}

// Status computes FR-8's reading status: 0.0 -> NotStarted, 1.0 ->
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
