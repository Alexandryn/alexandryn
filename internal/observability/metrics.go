package observability

import (
	"context"
	"encoding/json"
	"expvar"
	"sort"
	"strconv"
	"sync"
	"time"
)

// DefaultLatencyBuckets defines histogram upper bounds in milliseconds.
// Observations above the last bound land in the "+Inf" overflow bucket.
var DefaultLatencyBuckets = []float64{
	1, 2.5, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000,
}

// overflowBucketLabel is the key for observations above the last finite
// bucket bound.
const overflowBucketLabel = "+Inf"

// LatencySnapshot represents computed percentiles and counts for a route.
type LatencySnapshot struct {
	Count int64   `json:"count"`
	SumMS float64 `json:"sum_ms"`
	P50MS float64 `json:"p50_ms"`
	P95MS float64 `json:"p95_ms"`
	P99MS float64 `json:"p99_ms"`
	// Buckets maps each upper-bound label (in ms, and "+Inf" for the
	// overflow bucket) to its cumulative-in-that-band observation count.
	Buckets map[string]int64 `json:"buckets"`
}

// LatencyHistogram tracks observed request durations.
type LatencyHistogram struct {
	mu       sync.RWMutex
	buckets  []float64
	counts   []int64
	overflow int64
	samples  []float64
	count    int64
	sumMS    float64
}

// NewLatencyHistogram constructs a histogram with default latency buckets.
func NewLatencyHistogram() *LatencyHistogram {
	return &LatencyHistogram{
		buckets: DefaultLatencyBuckets,
		counts:  make([]int64, len(DefaultLatencyBuckets)),
		samples: make([]float64, 0, 1024),
	}
}

// Observe records a duration observation.
func (h *LatencyHistogram) Observe(d time.Duration) {
	ms := float64(d.Microseconds()) / 1000.0
	h.mu.Lock()
	defer h.mu.Unlock()

	h.count++
	h.sumMS += ms

	matched := false
	for i, b := range h.buckets {
		if ms <= b {
			h.counts[i]++
			matched = true
			break
		}
	}
	if !matched {
		h.overflow++
	}

	// Keep bounded recent samples for quantile estimation
	if len(h.samples) < 2048 {
		h.samples = append(h.samples, ms)
	} else {
		// Note: Bounded round-robin replacement when capacity is reached.
		// While not uniformly random once the buffer wraps around, this preserves
		// a rolling window of recent samples for operational p50/p95/p99 latency estimation.
		h.samples[h.count%int64(len(h.samples))] = ms
	}
}

// Snapshot computes current count, sum, and quantile estimates.
func (h *LatencyHistogram) Snapshot() LatencySnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()

	snap := LatencySnapshot{
		Count:   h.count,
		SumMS:   h.sumMS,
		Buckets: make(map[string]int64, len(h.buckets)+1),
	}
	for i, b := range h.buckets {
		snap.Buckets[strconv.FormatFloat(b, 'f', -1, 64)] = h.counts[i]
	}
	snap.Buckets[overflowBucketLabel] = h.overflow

	if len(h.samples) == 0 {
		return snap
	}

	sorted := make([]float64, len(h.samples))
	copy(sorted, h.samples)
	sort.Float64s(sorted)

	snap.P50MS = percentile(sorted, 0.50)
	snap.P95MS = percentile(sorted, 0.95)
	snap.P99MS = percentile(sorted, 0.99)
	return snap
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// PoolStats holds PostgreSQL connection pool statistics.
type PoolStats struct {
	AcquiredConns int32 `json:"acquired_conns"`
	IdleConns     int32 `json:"idle_conns"`
	TotalConns    int32 `json:"total_conns"`
	MaxConns      int32 `json:"max_conns"`
}

// MetricsSnapshot represents the full operational state exported to diagnostics.
type MetricsSnapshot struct {
	Latencies  map[string]LatencySnapshot `json:"latencies"`
	QueueDepth map[string]int             `json:"queue_depth"`
	DBPool     PoolStats                  `json:"db_pool"`
}

// PoolStatsProvider is a func returning current pool statistics.
type PoolStatsProvider func() PoolStats

// QueueDepthProvider is a func returning current queue counts per state.
type QueueDepthProvider func(ctx context.Context) (map[string]int, error)

// Registry manages in-process expvar metrics.
type Registry struct {
	mu        sync.RWMutex
	routes    map[string]*LatencyHistogram
	poolFn    PoolStatsProvider
	queueFn   QueueDepthProvider
	expvarMap *expvar.Map
}

// NewRegistry constructs a new operational metrics registry.
func NewRegistry() *Registry {
	var m *expvar.Map
	if v := expvar.Get("alexandryn_metrics"); v != nil {
		if existing, ok := v.(*expvar.Map); ok {
			m = existing
		}
	}
	if m == nil {
		m = expvar.NewMap("alexandryn_metrics")
	}
	return &Registry{
		routes:    make(map[string]*LatencyHistogram),
		expvarMap: m,
	}
}

// SetPoolStatsProvider sets the callback for pool stats.
func (r *Registry) SetPoolStatsProvider(fn PoolStatsProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.poolFn = fn
}

// SetQueueDepthProvider sets the callback for background queue depths.
func (r *Registry) SetQueueDepthProvider(fn QueueDepthProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queueFn = fn
}

// ObserveRequest records a request duration for a route template.
func (r *Registry) ObserveRequest(route string, d time.Duration) {
	r.mu.Lock()
	h, ok := r.routes[route]
	if !ok {
		h = NewLatencyHistogram()
		r.routes[route] = h
	}
	r.mu.Unlock()

	h.Observe(d)
}

// Snapshot gathers point-in-time metrics across routes, queue, and database pool.
// It returns a non-nil error only if unrecoverable provider failures occur;
// transient queue depth retrieval errors are handled with empty fallback data
// so operational metrics collection degrades gracefully.
func (r *Registry) Snapshot(ctx context.Context) (MetricsSnapshot, error) {
	r.mu.RLock()
	routesCopy := make(map[string]*LatencyHistogram, len(r.routes))
	for k, v := range r.routes {
		routesCopy[k] = v
	}
	poolFn := r.poolFn
	queueFn := r.queueFn
	r.mu.RUnlock()

	latencies := make(map[string]LatencySnapshot, len(routesCopy))
	for route, h := range routesCopy {
		latencies[route] = h.Snapshot()
	}

	var pool PoolStats
	if poolFn != nil {
		pool = poolFn()
	}

	queue := make(map[string]int)
	if queueFn != nil {
		var err error
		queue, err = queueFn(ctx)
		if err != nil {
			// Do not fail entire snapshot on queue depth query error
			queue = map[string]int{}
		}
	}

	result := MetricsSnapshot{
		Latencies:  latencies,
		QueueDepth: queue,
		DBPool:     pool,
	}

	// Publish the current snapshot into expvar, refreshed at snapshot time.
	r.expvarMap.Set("latencies", jsonVar{latencies})
	r.expvarMap.Set("queue_depth", jsonVar{queue})
	r.expvarMap.Set("db_pool", jsonVar{pool})

	return result, nil
}

// jsonVar adapts an arbitrary value to expvar.Var by marshalling it to
// JSON on read, so a struct or map can be published without a bespoke
// expvar type per shape.
type jsonVar struct{ v any }

func (j jsonVar) String() string {
	b, err := json.Marshal(j.v)
	if err != nil {
		return "null"
	}
	return string(b)
}
