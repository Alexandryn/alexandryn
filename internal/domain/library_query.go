package domain

import (
	"encoding/base64"
	"strings"
	"time"
)

// LibraryFilter represents the closed filter vocabulary (backend-library-api.md FR-3).
type LibraryFilter string

const (
	FilterOwned  LibraryFilter = "owned"
	FilterWanted LibraryFilter = "wanted"
	FilterAll    LibraryFilter = "all"
)

// LibrarySort represents the closed sort vocabulary (backend-library-api.md FR-4).
type LibrarySort string

const (
	SortAddedAt LibrarySort = "added_at"
	SortTitle   LibrarySort = "title"
)

// LibraryQuery holds validated query parameters for GET /api/v1/library.
type LibraryQuery struct {
	Cursor string
	Limit  int
	Q      string
	Filter LibraryFilter
	Sort   LibrarySort
	// LibraryID is the active library the request is scoped to. Empty
	// means the default library. Every library_entries / collection
	// reference in the resulting query is filtered by it (audit 0016 #88).
	LibraryID LibraryID
}

// WorkSummary is one Work in the /api/v1/library response list.
type WorkSummary struct {
	ID          WorkID
	Title       string
	Subtitle    string
	Authors     []string
	IsOwned     bool
	Collections []CollectionRef
	AddedAt     *time.Time
}

// CollectionRef is an embedded reference to a Collection with per-membership added_at.
type CollectionRef struct {
	ID      CollectionID
	Name    string
	AddedAt *time.Time
}

// LibraryPage is the response model for library queries.
type LibraryPage struct {
	Works      []*WorkSummary
	NextCursor string
}

// OwnedEdition represents an owned Edition in a WorkDetail response (FR-5).
type OwnedEdition struct {
	ID              EditionID
	Language        string
	ISBN            *string
	Publisher       string
	PublicationYear *int
	AddedAt         *time.Time
	Formats         []string
}

// WorkDetail is the complete detail model for GET /api/v1/works/:id.
type WorkDetail struct {
	ID               WorkID
	Title            string
	Subtitle         string
	Authors          []string
	Subjects         []string
	OriginalLanguage *Language
	OwnedEditions    []OwnedEdition
	Collections      []CollectionRef
}

// EncodeAddedAtCursor encodes (added_at, id) into an opaque base64 string (FR-1, FR-4).
func EncodeAddedAtCursor(addedAt time.Time, id WorkID) string {
	payload := addedAt.Format(time.RFC3339Nano) + "\x00" + string(id)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// DecodeAddedAtCursor decodes an opaque base64 cursor into (added_at, id).
func DecodeAddedAtCursor(cursor string) (time.Time, WorkID, error) {
	if cursor == "" {
		return time.Time{}, "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	parts := strings.Split(string(b), "\x00")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return time.Time{}, "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	return t, WorkID(parts[1]), nil
}

// EncodeTitleCursor encodes (title, id) into an opaque base64 string (FR-1, FR-4).
func EncodeTitleCursor(title string, id WorkID) string {
	payload := title + "\x00" + string(id)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// DecodeTitleCursor decodes an opaque base64 cursor into (title, id).
func DecodeTitleCursor(cursor string) (string, WorkID, error) {
	if cursor == "" {
		return "", "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	parts := strings.Split(string(b), "\x00")
	if len(parts) != 2 || parts[1] == "" {
		return "", "", &Error{Category: InvalidInput, Message: "invalid cursor"}
	}
	return parts[0], WorkID(parts[1]), nil
}
