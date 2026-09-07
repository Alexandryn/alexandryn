package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/observability"
)

type activityEventsResponse struct {
	Events []observability.SystemEvent `json:"events"`
}

type activityPauseAllResponse struct {
	Paused bool `json:"paused"`
}

type activityJobCancelResponse struct {
	Cancelled bool `json:"cancelled"`
}

type activityJobRetryResponse struct {
	NewJobID string `json:"new_job_id"`
}

type activityClearCompletedResponse struct {
	ClearedCount int64 `json:"cleared_count"`
}

// ActivityEventsHandler handles GET /api/v1/activity/events (FR-11).
func ActivityEventsHandler(eventStore *observability.EventStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		corrID := CorrelationIDFromContext(ctx)

		if eventStore == nil {
			WriteError(w, domain.Unavailable, "event store not ready", corrID)
			return
		}

		limit := 50
		if qLimit := r.URL.Query().Get("limit"); qLimit != "" {
			if parsed, err := strconv.Atoi(qLimit); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 100 {
			limit = 100
		}

		activeLib := ActiveLibraryFromContext(ctx)
		var libIDPtr *string
		if activeLib != "" {
			s := string(activeLib)
			libIDPtr = &s
		}

		events, err := eventStore.ListEvents(ctx, libIDPtr, limit)
		if err != nil {
			WriteError(w, domain.Internal, "failed to query system events", corrID)
			return
		}
		if events == nil {
			events = []observability.SystemEvent{}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(activityEventsResponse{Events: events})
	})
}

// ActivityPauseAllHandler handles POST /api/v1/activity/pause-all (FR-12).
func ActivityPauseAllHandler(sys *jobs.System) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sys != nil {
			sys.Pause()
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(activityPauseAllResponse{Paused: true})
	})
}

// ActivityJobCancelHandler handles POST /api/v1/activity/jobs/{id}/cancel (FR-12).
func ActivityJobCancelHandler(queue *jobs.Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		corrID := CorrelationIDFromContext(ctx)

		if queue == nil {
			WriteError(w, domain.Unavailable, "job queue not ready", corrID)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			WriteError(w, domain.InvalidInput, "job id required", corrID)
			return
		}

		err := queue.CancelJob(ctx, jobs.ID(id))
		if err != nil {
			var dErr *domain.Error
			if errors.As(err, &dErr) {
				WriteError(w, dErr.Category, dErr.Message, corrID)
				return
			}
			WriteError(w, domain.Internal, "failed to cancel job", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(activityJobCancelResponse{Cancelled: true})
	})
}

// ActivityJobRetryHandler handles POST /api/v1/activity/jobs/{id}/retry (FR-12).
func ActivityJobRetryHandler(queue *jobs.Queue) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		corrID := CorrelationIDFromContext(ctx)

		if queue == nil {
			WriteError(w, domain.Unavailable, "job queue not ready", corrID)
			return
		}

		id := r.PathValue("id")
		if id == "" {
			WriteError(w, domain.InvalidInput, "job id required", corrID)
			return
		}

		newID, err := queue.RetryJob(ctx, jobs.ID(id))
		if err != nil {
			var dErr *domain.Error
			if errors.As(err, &dErr) {
				WriteError(w, dErr.Category, dErr.Message, corrID)
				return
			}
			WriteError(w, domain.Internal, "failed to retry job", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(activityJobRetryResponse{NewJobID: string(newID)})
	})
}

// ActivityClearCompletedHandler handles POST /api/v1/activity/jobs/clear-completed (FR-12).
func ActivityClearCompletedHandler(queue *jobs.Queue, clock func() time.Time) http.Handler {
	if clock == nil {
		clock = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		corrID := CorrelationIDFromContext(ctx)

		if queue == nil {
			WriteError(w, domain.Unavailable, "job queue not ready", corrID)
			return
		}

		cutoff := clock().Add(-1 * time.Hour)
		cleared, err := queue.ClearCompleted(ctx, cutoff)
		if err != nil {
			var dErr *domain.Error
			if errors.As(err, &dErr) {
				WriteError(w, dErr.Category, dErr.Message, corrID)
				return
			}
			WriteError(w, domain.Internal, "failed to clear completed jobs", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(activityClearCompletedResponse{ClearedCount: cleared})
	})
}
