package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type stubIDs struct{ id string }

func (s stubIDs) NewID() string { return s.id }

func TestQueue_Enqueue_UnregisteredKindIsInvalidInput(t *testing.T) {
	// store is nil: an unregistered kind must be rejected before the
	// store is ever touched.
	q := NewQueue(nil, NewRegistry(), stubIDs{id: "x"}, fixedClock{})

	_, err := q.Enqueue(context.Background(), "never-registered", map[string]int{"a": 1})
	if domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("category = %v, want InvalidInput", domain.CategoryOf(err))
	}
}

func TestQueue_Enqueue_NonSerializablePayloadIsInvalidInput(t *testing.T) {
	r := NewRegistry()
	r.Register("k", 3, noopHandler)
	q := NewQueue(nil, r, stubIDs{id: "x"}, fixedClock{})

	_, err := q.Enqueue(context.Background(), "k", make(chan int))
	if domain.CategoryOf(err) != domain.InvalidInput {
		t.Fatalf("category = %v, want InvalidInput for an unmarshalable payload", domain.CategoryOf(err))
	}
}
