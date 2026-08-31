package domain

// CollectionSummary represents one collection with its work count (backend-library-api.md FR-6).
type CollectionSummary struct {
	ID        CollectionID
	Name      string
	WorkCount int
}

// CollectionDetail represents a collection with its member works (backend-library-api.md FR-6).
type CollectionDetail struct {
	ID    CollectionID
	Name  string
	Works []*WorkSummary
}
