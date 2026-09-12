package domain

import "strings"

// FileReference is a constrained value type representing an opaque,
// Source-scoped identifier plus a declared format and an optional size in bytes.
// It is treated as an opaque token in the domain and never directly interpreted
// as a local filesystem path or URL without adapter validation.
type FileReference struct {
	ReferenceID string
	Format      string
	SizeBytes   *int64
}

// NewFileReference requires both ReferenceID and Format to be non-empty.
// ReferenceID is source-defined and opaque by design.
func NewFileReference(referenceID, format string, sizeBytes *int64) (FileReference, error) {
	if strings.TrimSpace(referenceID) == "" {
		return FileReference{}, &Error{Category: InvalidInput, Message: "file reference id must not be empty"}
	}
	if strings.TrimSpace(format) == "" {
		return FileReference{}, &Error{Category: InvalidInput, Message: "file reference format must not be empty"}
	}
	return FileReference{ReferenceID: referenceID, Format: format, SizeBytes: sizeBytes}, nil
}
