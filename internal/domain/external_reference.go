package domain

// ExternalReference is an optional, attached pointer to another catalog's
// record of a Work/Edition/Author — never the primary key
// (domain-bibliographic.md FR-1/FR-2/FR-7). Source is a fixed, extensible
// vocabulary ("openlibrary" today); ID is that source's own identifier
// format, opaque to this domain.
type ExternalReference struct {
	Source string
	ID     string
}
