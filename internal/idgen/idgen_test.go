package idgen_test

import (
	"regexp"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestGenerator_NewID_ProducesRFC4122UUIDv4Shape(t *testing.T) {
	g := idgen.New()
	for i := 0; i < 100; i++ {
		id := g.NewID()
		if !uuidV4Pattern.MatchString(id) {
			t.Fatalf("NewID() = %q, does not match UUID v4 shape (version nibble 4, variant 8/9/a/b)", id)
		}
	}
}

func TestGenerator_NewID_IsUnique(t *testing.T) {
	g := idgen.New()
	seen := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := g.NewID()
		if seen[id] {
			t.Fatalf("NewID() produced a duplicate: %q", id)
		}
		seen[id] = true
	}
}

// Generator must satisfy domain.IDGenerator.
var _ domain.IDGenerator = (*idgen.Generator)(nil)
