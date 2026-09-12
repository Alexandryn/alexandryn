package jobs

import (
	"fmt"
	"sort"
	"sync"
)

// registration is one kind's registered handler and its attempt budget.
type registration struct {
	maxAttempts int
	handler     HandlerFunc
}

// Registry maps a job kind to its handler. It is an in-process map, not
// a database table: handlers are Go code compiled into this binary.
// Registration happens once at process startup; lookups happen on every claim.
type Registry struct {
	mu      sync.RWMutex
	entries map[Kind]registration
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[Kind]registration)}
}

// Register binds a handler to a kind with a per-job attempt limit. It
// panics on a misconfiguration that can only be a programming error
// caught at startup: an empty kind, a nil handler, maxAttempts below 1,
// or a kind already registered.
func (r *Registry) Register(kind Kind, maxAttempts int, handler HandlerFunc) {
	if kind == "" {
		panic("jobs: Register called with an empty kind")
	}
	if handler == nil {
		panic(fmt.Sprintf("jobs: Register(%q) called with a nil handler", kind))
	}
	if maxAttempts < 1 {
		panic(fmt.Sprintf("jobs: Register(%q) called with maxAttempts %d, must be >= 1", kind, maxAttempts))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[kind]; exists {
		panic(fmt.Sprintf("jobs: kind %q is already registered", kind))
	}
	r.entries[kind] = registration{maxAttempts: maxAttempts, handler: handler}
}

func (r *Registry) lookup(kind Kind) (registration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.entries[kind]
	return reg, ok
}

// kinds returns every registered kind, sorted for a stable claim-query
// argument. An empty result means the engine claims nothing — there is
// no handler to run anything.
func (r *Registry) kinds() []Kind {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Kind, 0, len(r.entries))
	for k := range r.entries {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
