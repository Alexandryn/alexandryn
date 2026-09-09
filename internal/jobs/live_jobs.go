package jobs

import (
	"context"
	"sync"
)

// liveJobs tracks the cancel function of every job a worker in this
// process is currently executing. An admin cancel writes dead_letter to
// the row and then aborts the handler here immediately, instead of
// leaving it running until the next heartbeat tick notices the row moved
// (audit 0016 #299). All methods are nil-safe so a Queue or Engine built
// without one (tests) still works.
type liveJobs struct {
	mu      sync.Mutex
	cancels map[ID]context.CancelFunc
}

func newLiveJobs() *liveJobs {
	return &liveJobs{cancels: make(map[ID]context.CancelFunc)}
}

func (l *liveJobs) add(id ID, cancel context.CancelFunc) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.cancels[id] = cancel
	l.mu.Unlock()
}

func (l *liveJobs) remove(id ID) {
	if l == nil {
		return
	}
	l.mu.Lock()
	delete(l.cancels, id)
	l.mu.Unlock()
}

// cancel aborts the handler for id if it is executing in this process,
// and reports whether it was.
func (l *liveJobs) cancel(id ID) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	cancel, ok := l.cancels[id]
	l.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}
