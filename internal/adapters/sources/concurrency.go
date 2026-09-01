package sources

// Semaphore is the global cap on outbound source requests
// (backend-source-adapter.md FR-14): at most 50 in flight across every
// configured source combined — health checks, browse, search, and
// continuation-link fetches alike. Unlike the metadata adapter's shared
// rate limiter (one external service with a stated policy), sources are
// independent hosts with no shared external ceiling, so this exists
// purely to bound this system's own goroutines and connections. A
// request arriving at the cap is rejected immediately, never queued —
// there is no external limit to wait out.
type Semaphore struct {
	slots chan struct{}
}

// DefaultOutboundLimit is FR-14's figure.
const DefaultOutboundLimit = 50

// NewSemaphore builds a Semaphore with n slots. n <= 0 is treated as
// DefaultOutboundLimit.
func NewSemaphore(n int) *Semaphore {
	if n <= 0 {
		n = DefaultOutboundLimit
	}
	return &Semaphore{slots: make(chan struct{}, n)}
}

// TryAcquire takes a slot without blocking. It returns false when the
// cap is already reached — the caller then returns 503 Unavailable
// immediately.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release returns a slot. It must be called exactly once for each
// TryAcquire that returned true.
func (s *Semaphore) Release() {
	select {
	case <-s.slots:
	default:
		// Release without a matching successful acquire — a caller bug.
		// Do not block or panic in a request path; drop it.
	}
}

// InFlight returns the current number of held slots. For tests and
// observability, not flow control.
func (s *Semaphore) InFlight() int { return len(s.slots) }
