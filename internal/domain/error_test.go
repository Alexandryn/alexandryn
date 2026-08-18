package domain_test

import (
	"errors"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// FR-1: the six categories are the closed set.
func TestCategories_AreTheSixNamedByFR1(t *testing.T) {
	want := []domain.Category{
		domain.NotFound,
		domain.InvalidInput,
		domain.Unauthorized,
		domain.Conflict,
		domain.Unavailable,
		domain.Internal,
	}
	seen := map[domain.Category]bool{}
	for _, c := range want {
		if seen[c] {
			t.Fatalf("category %q listed twice", c)
		}
		seen[c] = true
	}
	if len(seen) != 6 {
		t.Fatalf("got %d distinct categories, want exactly 6", len(seen))
	}
}

func TestError_ConstructsWithCategoryAndMessage(t *testing.T) {
	err := &domain.Error{Category: domain.NotFound, Message: "work not found"}
	if err.Category != domain.NotFound {
		t.Fatalf("Category = %v, want NotFound", err.Category)
	}
	if err.Error() != "work not found" {
		t.Fatalf("Error() = %q, want %q", err.Error(), "work not found")
	}
}

func TestError_WrapsAnUnderlyingErrorForServerSideDetail(t *testing.T) {
	underlying := errors.New("pq: duplicate key value violates unique constraint")
	err := &domain.Error{Category: domain.Conflict, Message: "already exists", Err: underlying}

	if !errors.Is(err, underlying) {
		t.Fatal("errors.Is(err, underlying) = false, want true — Error must unwrap to the underlying cause")
	}
}

// FR-4: an error not already carrying a category defaults to Internal —
// nothing reaches the client without going through one of the six.
func TestCategoryOf_DomainErrorReturnsItsOwnCategory(t *testing.T) {
	err := &domain.Error{Category: domain.Conflict, Message: "already exists"}
	if got := domain.CategoryOf(err); got != domain.Conflict {
		t.Fatalf("CategoryOf(domain.Error) = %v, want Conflict", got)
	}
}

func TestCategoryOf_WrappedDomainErrorReturnsItsCategory(t *testing.T) {
	inner := &domain.Error{Category: domain.Unavailable, Message: "database unreachable"}
	wrapped := errors.New("startup: " + inner.Error())
	wrapped = errors.Join(wrapped, inner) // still resolvable via errors.As through the chain

	if got := domain.CategoryOf(wrapped); got != domain.Unavailable {
		t.Fatalf("CategoryOf(wrapped) = %v, want Unavailable", got)
	}
}

func TestCategoryOf_UncategorizedErrorDefaultsToInternal(t *testing.T) {
	plain := errors.New("unexpected panic recovered")
	if got := domain.CategoryOf(plain); got != domain.Internal {
		t.Fatalf("CategoryOf(plain error) = %v, want Internal (FR-4's default)", got)
	}
}

func TestCategoryOf_NilErrorDefaultsToInternal(t *testing.T) {
	if got := domain.CategoryOf(nil); got != domain.Internal {
		t.Fatalf("CategoryOf(nil) = %v, want Internal", got)
	}
}
