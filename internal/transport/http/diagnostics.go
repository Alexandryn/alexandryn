package http

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"

	"github.com/Alexandryn/alexandryn/internal/observability"
)

type diagnosticsResponse struct {
	UptimeSeconds int64                         `json:"uptime_seconds"`
	Version       diagnosticsVersion            `json:"version"`
	Runtime       diagnosticsRuntime            `json:"runtime"`
	Metrics       observability.MetricsSnapshot `json:"metrics"`
}

type diagnosticsVersion struct {
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

type diagnosticsRuntime struct {
	Goroutines      int    `json:"goroutines"`
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes"`
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	GCCycles        uint32 `json:"gc_cycles"`
}

// DiagnosticsHandler returns an HTTP handler serving GET /api/v1/diagnostics.
func DiagnosticsHandler(reg *observability.Registry, startTime time.Time, commit, buildTime string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		uptime := int64(time.Since(startTime).Seconds())

		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)

		var snap observability.MetricsSnapshot
		if reg != nil {
			var err error
			snap, err = reg.Snapshot(ctx)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(errorBody{
					Code:          "Internal",
					Message:       "failed to collect metrics snapshot",
					CorrelationID: CorrelationIDFromContext(ctx),
				})
				return
			}
		}

		resp := diagnosticsResponse{
			UptimeSeconds: uptime,
			Version: diagnosticsVersion{
				Commit:    commit,
				BuildTime: buildTime,
			},
			Runtime: diagnosticsRuntime{
				Goroutines:      runtime.NumGoroutine(),
				HeapAllocBytes:  memStats.HeapAlloc,
				TotalAllocBytes: memStats.TotalAlloc,
				GCCycles:        memStats.NumGC,
			},
			Metrics: snap,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
