package domain_test

import "testing"

import "github.com/Alexandryn/alexandryn/internal/domain"

// domain-source.md FR-4: a FileReference MUST be a constrained type, not
// a bare string — an opaque, Source-scoped identifier plus a declared
// format and an optional size.
func TestNewFileReference(t *testing.T) {
	t.Run("valid reference", func(t *testing.T) {
		size := int64(1024)
		_, err := domain.NewFileReference("ref-1", "epub", &size)
		if err != nil {
			t.Fatalf("NewFileReference: %v", err)
		}
	})
	t.Run("nil size is legal (unknown is legal)", func(t *testing.T) {
		ref, err := domain.NewFileReference("ref-1", "epub", nil)
		if err != nil {
			t.Fatalf("NewFileReference with nil size: %v", err)
		}
		if ref.SizeBytes != nil {
			t.Fatalf("SizeBytes = %v, want nil", ref.SizeBytes)
		}
	})
	t.Run("empty reference id rejected", func(t *testing.T) {
		_, err := domain.NewFileReference("", "epub", nil)
		if err == nil {
			t.Fatal("NewFileReference(\"\", ...) = nil error, want an error")
		}
	})
	t.Run("empty format rejected", func(t *testing.T) {
		_, err := domain.NewFileReference("ref-1", "", nil)
		if err == nil {
			t.Fatal("NewFileReference(..., \"\", ...) = nil error, want an error")
		}
	})
}
