package testutil

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

type spySink struct {
	mu      sync.Mutex
	records []slog.Record
}

// SpyHandler is an slog.Handler that captures every record it receives
// instead of writing it anywhere, so a test can assert on log output
// directly — including proving a value never appears in it.
type SpyHandler struct {
	sink   *spySink
	attrs  []slog.Attr
	groups []string
}

// NewSpyHandler constructs an empty SpyHandler.
func NewSpyHandler() *SpyHandler {
	return &SpyHandler{sink: &spySink{}}
}

// Enabled reports true for every level — a spy captures everything a test
// sends it and lets the test itself decide what matters.
func (h *SpyHandler) Enabled(context.Context, slog.Level) bool { return true }

// Handle captures r, merging in any attrs/groups bound via With/WithGroup.
func (h *SpyHandler) Handle(_ context.Context, r slog.Record) error {
	clone := r.Clone()
	if merged := h.mergedAttrs(); len(merged) > 0 {
		clone.AddAttrs(merged...)
	}

	h.sink.mu.Lock()
	h.sink.records = append(h.sink.records, clone)
	h.sink.mu.Unlock()
	return nil
}

// WithAttrs returns a derived handler that merges attrs into every record
// it captures, sharing this handler's underlying capture sink.
func (h *SpyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	next := *h
	next.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &next
}

// WithGroup returns a derived handler that nests subsequently bound attrs
// under name, sharing this handler's underlying capture sink.
func (h *SpyHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	next.groups = append(append([]string{}, h.groups...), name)
	return &next
}

func (h *SpyHandler) mergedAttrs() []slog.Attr {
	if len(h.attrs) == 0 {
		return nil
	}
	attrs := append([]slog.Attr(nil), h.attrs...)
	for i := len(h.groups) - 1; i >= 0; i-- {
		args := make([]any, len(attrs))
		for j, a := range attrs {
			args[j] = a
		}
		attrs = []slog.Attr{slog.Group(h.groups[i], args...)}
	}
	return attrs
}

// Records returns every record captured so far, oldest first.
func (h *SpyHandler) Records() []slog.Record {
	h.sink.mu.Lock()
	defer h.sink.mu.Unlock()
	out := make([]slog.Record, len(h.sink.records))
	copy(out, h.sink.records)
	return out
}

// Contains reports whether substr appears in any captured record's
// message or any attr's string value — the shape a "known secret never
// reaches the logs" test needs.
func (h *SpyHandler) Contains(substr string) bool {
	for _, r := range h.Records() {
		if strings.Contains(r.Message, substr) {
			return true
		}
		found := false
		r.Attrs(func(a slog.Attr) bool {
			if strings.Contains(a.Value.String(), substr) {
				found = true
				return false
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}
