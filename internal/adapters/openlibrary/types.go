package openlibrary

import "context"

// NormalisedAuthor represents a normalised author reference.
type NormalisedAuthor struct {
	OpenLibraryAuthorKey *string `json:"openLibraryAuthorKey"`
	Name                 string  `json:"name"`
}

// NormalisedSearchResult represents a single search result item in /api/v1/discover.
type NormalisedSearchResult struct {
	OpenLibraryWorkKey string             `json:"openLibraryWorkKey"`
	Title              string             `json:"title"`
	Authors            []NormalisedAuthor `json:"authors"`
	FirstPublishYear   *int               `json:"firstPublishYear"`
	CoverURL           *string            `json:"coverUrl"`
	EditionCount       int                `json:"editionCount"`
}

// NormalisedSearchResponse represents the paginated search response.
type NormalisedSearchResponse struct {
	Items  []NormalisedSearchResult `json:"items"`
	Total  int                      `json:"total"`
	Limit  int                      `json:"limit"`
	Offset int                      `json:"offset"`
}

// NormalisedWork represents normalised work details.
type NormalisedWork struct {
	Title       string             `json:"title"`
	Subtitle    string             `json:"subtitle"`
	Description string             `json:"description"`
	Subjects    []string           `json:"subjects"`
	Authors     []NormalisedAuthor `json:"authors"`
	CoverURL    *string            `json:"coverUrl"`
}

// NormalisedEdition represents one edition of an Open Library work.
type NormalisedEdition struct {
	Title                 string  `json:"title"`
	Publisher             string  `json:"publisher"`
	PublishDate           string  `json:"publishDate"`
	Language              string  `json:"language"`
	OpenLibraryEditionKey string  `json:"openLibraryEditionKey"`
	CoverURL              *string `json:"coverUrl"`
}

// DiscoverWorkDetail represents the full response for a work detail lookup.
type DiscoverWorkDetail struct {
	Work     NormalisedWork      `json:"work"`
	Editions []NormalisedEdition `json:"editions"`
}

// Client defines the interface for interacting with Open Library.
type Client interface {
	Search(ctx context.Context, q string, limit, offset int) (*NormalisedSearchResponse, error)
	GetWork(ctx context.Context, openLibraryID string) (*DiscoverWorkDetail, error)
	FetchCover(ctx context.Context, coverID int64) (data []byte, contentType string, err error)
}
