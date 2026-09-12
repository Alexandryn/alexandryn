package domain

// ExternalReference is an optional, attached pointer to another catalog's
// record of a Work, Edition, or Author, and is never used as the primary key.
// Source identifies the external catalog (e.g. "openlibrary"); ID is that
// source's own identifier format, opaque to this domain.
type ExternalReference struct {
	Source string
	ID     string
}
