package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// A Collection contains Works, not Editions or files. An empty Collection
// (zero Works) is a valid, ordinary initial state.
func TestNewCollection(t *testing.T) {
	tests := []struct {
		name       string
		collection string
		wantErr    bool
	}{
		{"valid name", "Want to Read", false},
		{"empty name rejected", "", true},
		{"over-length name rejected", strings.Repeat("a", 101), true},
		{"control character rejected", "Evil\x00Name", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewCollection(domain.CollectionID("collection-1"), tt.collection)
			if tt.wantErr && err == nil {
				t.Fatalf("NewCollection(%q) = nil error, want an error", tt.collection)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NewCollection(%q) = %v, want nil", tt.collection, err)
			}
		})
	}
}

func TestNewCollection_IsLegallyEmpty(t *testing.T) {
	c, err := domain.NewCollection(domain.CollectionID("collection-1"), "Want to Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	if len(c.Members()) != 0 {
		t.Fatalf("Members() = %v, want empty", c.Members())
	}
}

// Collection membership carries its own added-at timestamp per Work,
// independent of any LibraryEntry's.
func TestCollection_AddMember(t *testing.T) {
	c, err := domain.NewCollection(domain.CollectionID("collection-1"), "Want to Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	now := time.Now()
	c.AddMember(domain.WorkID("work-1"), now)

	if len(c.Members()) != 1 {
		t.Fatalf("Members() = %v, want 1", c.Members())
	}
	if c.Members()[0].WorkID != "work-1" || !c.Members()[0].AddedAt.Equal(now) {
		t.Fatalf("Members()[0] = %+v, want {work-1, %v}", c.Members()[0], now)
	}
}

func TestCollection_RemoveMember(t *testing.T) {
	c, err := domain.NewCollection(domain.CollectionID("collection-1"), "Want to Read")
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	now := time.Now()
	c.AddMember(domain.WorkID("work-1"), now)
	c.AddMember(domain.WorkID("work-2"), now)
	c.RemoveMember(domain.WorkID("work-1"))

	if len(c.Members()) != 1 || c.Members()[0].WorkID != "work-2" {
		t.Fatalf("Members() after removal = %v, want [work-2]", c.Members())
	}
}
