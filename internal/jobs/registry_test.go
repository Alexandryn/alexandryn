package jobs

import (
	"context"
	"encoding/json"
	"testing"
)

func noopHandler(context.Context, json.RawMessage, ReportProgressFunc) error { return nil }

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register("import", 5, noopHandler)

	reg, ok := r.lookup("import")
	if !ok {
		t.Fatal("lookup(import) not found after Register")
	}
	if reg.maxAttempts != 5 {
		t.Fatalf("maxAttempts = %d, want 5", reg.maxAttempts)
	}
	if _, ok := r.lookup("unregistered"); ok {
		t.Fatal("lookup(unregistered) should not be found")
	}
}

func TestRegistry_KindsSorted(t *testing.T) {
	r := NewRegistry()
	r.Register("sync", 3, noopHandler)
	r.Register("import", 3, noopHandler)
	r.Register("cover", 3, noopHandler)

	got := r.kinds()
	want := []Kind{"cover", "import", "sync"}
	if len(got) != len(want) {
		t.Fatalf("kinds() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kinds() = %v, want %v", got, want)
		}
	}
}

func TestRegistry_PanicsOnMisconfiguration(t *testing.T) {
	cases := map[string]func(*Registry){
		"empty kind":       func(r *Registry) { r.Register("", 3, noopHandler) },
		"nil handler":      func(r *Registry) { r.Register("k", 3, nil) },
		"zero maxAttempts": func(r *Registry) { r.Register("k", 0, noopHandler) },
		"duplicate kind": func(r *Registry) {
			r.Register("k", 3, noopHandler)
			r.Register("k", 3, noopHandler)
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s: Register did not panic", name)
				}
			}()
			fn(NewRegistry())
		})
	}
}
