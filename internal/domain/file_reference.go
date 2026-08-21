package domain

import "strings"

// FileReference (domain-source.md FR-4) is a constrained type, not a bare
// string: an opaque, Source-scoped identifier plus a declared format and
// a size in bytes if known. It MUST NOT be interpretable as a filesystem
// path or URL by anything in this domain — resolving one into actual
// bytes is phase 08's adapter's job, with its own hostile-input handling;
// this domain never parses or constructs a path from one, and this type
// exposes nothing that would let it (no path-shaped methods, no joining,
// no interpretation of ReferenceID's content).
type FileReference struct {
	ReferenceID string
	Format      string
	SizeBytes   *int64
}

// NewFileReference requires both ReferenceID and Format to be non-empty
// — an opaque identifier still needs some value, and Format is what
// FR-2's uniqueness key partly depends on. Neither is validated for
// shape beyond that: ReferenceID is source-defined and opaque by design;
// Format's actual vocabulary is phase 10's to define.
func NewFileReference(referenceID, format string, sizeBytes *int64) (FileReference, error) {
	if strings.TrimSpace(referenceID) == "" {
		return FileReference{}, &Error{Category: InvalidInput, Message: "file reference id must not be empty"}
	}
	if strings.TrimSpace(format) == "" {
		return FileReference{}, &Error{Category: InvalidInput, Message: "file reference format must not be empty"}
	}
	return FileReference{ReferenceID: referenceID, Format: format, SizeBytes: sizeBytes}, nil
}
