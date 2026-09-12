package domain

// CollectionSummary represents one collection with its work count.
type CollectionSummary struct {
	ID        CollectionID
	Name      string
	WorkCount int
}

// CollectionDetail represents a collection with its member works.
type CollectionDetail struct {
	ID    CollectionID
	Name  string
	Works []*WorkSummary
}
