package observability_test

import (
	"context"
	"encoding/json"
	"expvar"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/observability"
)

func TestLatencyHistogram_Percentiles(t *testing.T) {
	h := observability.NewLatencyHistogram()

	// Record durations: 10ms, 20ms, ..., 100ms
	for i := 1; i <= 100; i++ {
		h.Observe(time.Duration(i) * time.Millisecond)
	}

	snap := h.Snapshot()
	if snap.Count != 100 {
		t.Fatalf("Count = %d, want 100", snap.Count)
	}
	expectedSum := (100 * 101 / 2) // 5050ms
	if snap.SumMS != float64(expectedSum) {
		t.Fatalf("SumMS = %f, want %d", snap.SumMS, expectedSum)
	}
	if snap.P50MS < 45 || snap.P50MS > 55 {
		t.Errorf("P50MS = %f, want ~50", snap.P50MS)
	}
	if snap.P95MS < 90 || snap.P95MS > 97 {
		t.Errorf("P95MS = %f, want ~95", snap.P95MS)
	}
	if snap.P99MS < 95 || snap.P99MS > 100 {
		t.Errorf("P99MS = %f, want ~99", snap.P99MS)
	}
}

// audit 0016 #302: an observation slower than the last finite bucket
// bound must land in the "+Inf" overflow bucket, not vanish.
func TestLatencyHistogram_OverflowBucket(t *testing.T) {
	h := observability.NewLatencyHistogram()
	h.Observe(50 * time.Millisecond)
	h.Observe(30 * time.Second) // above the 10s top bound

	snap := h.Snapshot()
	if snap.Buckets["+Inf"] != 1 {
		t.Errorf("+Inf bucket = %d, want 1", snap.Buckets["+Inf"])
	}
	if snap.Buckets["50"] != 1 {
		t.Errorf("50ms bucket = %d, want 1", snap.Buckets["50"])
	}
	var total int64
	for _, c := range snap.Buckets {
		total += c
	}
	if total != snap.Count {
		t.Errorf("bucket total %d != Count %d — an observation was lost", total, snap.Count)
	}
}

// audit 0016 #302: Snapshot publishes into the expvar map ADR 0030 names
// as the read mechanism.
func TestMetrics_Snapshot_PublishesToExpvar(t *testing.T) {
	m := observability.NewRegistry()
	m.ObserveRequest("GET /x", 5*time.Millisecond)
	if _, err := m.Snapshot(context.Background()); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	v := expvar.Get("alexandryn_metrics")
	if v == nil {
		t.Fatal("alexandryn_metrics expvar not registered")
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v.String()), &parsed); err != nil {
		t.Fatalf("expvar map is not valid JSON: %v", err)
	}
	for _, key := range []string{"latencies", "queue_depth", "db_pool"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("expvar map missing %q after Snapshot", key)
		}
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	m := observability.NewRegistry()

	m.ObserveRequest("/api/v1/library", 15*time.Millisecond)
	m.ObserveRequest("/api/v1/library", 25*time.Millisecond)
	m.ObserveRequest("/api/v1/works/{id}", 10*time.Millisecond)

	m.SetPoolStatsProvider(func() observability.PoolStats {
		return observability.PoolStats{
			AcquiredConns: 2,
			IdleConns:     8,
			TotalConns:    10,
			MaxConns:      20,
		}
	})

	m.SetQueueDepthProvider(func(ctx context.Context) (map[string]int, error) {
		return map[string]int{
			"queued":      3,
			"running":     1,
			"retrying":    0,
			"dead_letter": 0,
		}, nil
	})

	snap, err := m.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.DBPool.AcquiredConns != 2 || snap.DBPool.MaxConns != 20 {
		t.Errorf("DBPool = %+v, want Acquired=2, Max=20", snap.DBPool)
	}

	if snap.QueueDepth["queued"] != 3 || snap.QueueDepth["running"] != 1 {
		t.Errorf("QueueDepth = %+v", snap.QueueDepth)
	}

	libHist, ok := snap.Latencies["/api/v1/library"]
	if !ok || libHist.Count != 2 {
		t.Errorf("Latencies[/api/v1/library] = %+v", libHist)
	}
}
