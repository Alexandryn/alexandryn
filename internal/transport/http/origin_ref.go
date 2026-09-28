package http

import "sync/atomic"

type originRefs struct {
	allowedOrigins atomic.Pointer[[]string]
}

// SetAllowedOrigins atomically updates the dynamic allowed origins list.
// Called once the server listener is bound and its assigned port (ephemeral or configured)
// is finalized.
func (r *PoolRef) SetAllowedOrigins(origins []string) {
	cp := make([]string, len(origins))
	copy(cp, origins)
	r.origin.allowedOrigins.Store(&cp)
}

// GetAllowedOrigins retrieves the dynamically configured allowed origins list, if set.
func (r *PoolRef) GetAllowedOrigins() ([]string, bool) {
	ptr := r.origin.allowedOrigins.Load()
	if ptr == nil {
		return nil, false
	}
	return *ptr, true
}
