//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/observability"
	"github.com/Alexandryn/alexandryn/internal/testutil"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func seqIDs() *testutil.FakeIDGenerator {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("act-id-%d", i+1)
	}
	return testutil.NewFakeIDGenerator(ids...)
}

func TestActivityEventsHandler_TenantIsolation(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	eventStore := observability.NewEventStore(pool, time.Now)

	lib1 := "00000000-0000-0000-0000-000000000001"
	lib2 := "00000000-0000-0000-0000-000000000002"

	// Seed library 2 if not exists
	_, _ = pool.Exec(ctx, `INSERT INTO libraries (id, name, created_at, updated_at) VALUES ($1, 'Lib 2', now(), now()) ON CONFLICT DO NOTHING`, lib2)

	// Seed 3 events: one in lib1, one in lib2, one host-level
	_ = eventStore.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind: observability.EventJobEnqueued,
		LibraryID: &lib1,
		Payload:   map[string]any{"source": "lib1"},
	})
	_ = eventStore.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind: observability.EventJobEnqueued,
		LibraryID: &lib2,
		Payload:   map[string]any{"source": "lib2"},
	})
	_ = eventStore.RecordEvent(ctx, observability.NewSystemEvent{
		EventKind: observability.EventJobCompleted,
		LibraryID: nil,
		Payload:   map[string]any{"source": "host"},
	})

	handler := transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityEventsHandler(eventStore))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/activity/events", nil)
	req = req.WithContext(transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
		UserID: "admin-1", Role: domain.RoleAdmin,
	}))
	req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), domain.LibraryID(lib1)))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Events []observability.SystemEvent `json:"events"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal events: %v", err)
	}

	if len(resp.Events) < 2 {
		t.Fatalf("got %d events, want >= 2", len(resp.Events))
	}

	for _, ev := range resp.Events {
		if ev.LibraryID != nil && *ev.LibraryID == lib2 {
			t.Errorf("tenant isolation leak: received event belonging to library 2 (%s)", *ev.LibraryID)
		}
	}
}

func TestActivityJobActions_EndToEnd(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()
	now := time.Now().UTC()

	ids := seqIDs()
	store := jobs.NewStore(pool, ids, 60*time.Second)
	reg := jobs.NewRegistry()
	queue := jobs.NewQueue(store, reg, ids, jobs.SystemClock{})

	adminUser := &transporthttp.AuthenticatedUser{UserID: "admin-1", Role: domain.RoleAdmin}

	// 1. Enqueue job
	_ = store.Enqueue(ctx, jobs.NewJob{
		ID:          "act-job-1",
		Kind:        "test.sync",
		Payload:     []byte(`{}`),
		MaxAttempts: 3,
		AvailableAt: now,
		Now:         now,
	})

	// 2. Cancel job via endpoint
	cancelHandler := transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityJobCancelHandler(queue))
	cancelReq := httptest.NewRequest(http.MethodPost, "/api/v1/activity/jobs/act-job-1/cancel", nil)
	cancelReq.SetPathValue("id", "act-job-1")
	cancelReq = cancelReq.WithContext(transporthttp.WithUser(cancelReq.Context(), adminUser))
	cancelRec := httptest.NewRecorder()
	cancelHandler.ServeHTTP(cancelRec, cancelReq)

	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, want 200", cancelRec.Code)
	}

	j, err := store.GetJob(ctx, "act-job-1")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if j.State != jobs.StateDeadLetter {
		t.Errorf("job state = %v, want dead_letter", j.State)
	}

	// 3. Retry dead-lettered job via endpoint
	retryHandler := transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityJobRetryHandler(queue))
	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/activity/jobs/act-job-1/retry", nil)
	retryReq.SetPathValue("id", "act-job-1")
	retryReq = retryReq.WithContext(transporthttp.WithUser(retryReq.Context(), adminUser))
	retryRec := httptest.NewRecorder()
	retryHandler.ServeHTTP(retryRec, retryReq)

	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry status = %d, want 200", retryRec.Code)
	}

	var retryResp struct {
		NewJobID string `json:"new_job_id"`
	}
	if err := json.Unmarshal(retryRec.Body.Bytes(), &retryResp); err != nil {
		t.Fatalf("unmarshal retry: %v", err)
	}
	if retryResp.NewJobID == "" {
		t.Fatal("expected non-empty new_job_id")
	}

	newJ, err := store.GetJob(ctx, jobs.ID(retryResp.NewJobID))
	if err != nil {
		t.Fatalf("GetJob(new): %v", err)
	}
	if newJ.State != jobs.StateQueued {
		t.Errorf("new job state = %v, want queued", newJ.State)
	}

	// 4. Clear completed jobs
	_ = store.Enqueue(ctx, jobs.NewJob{
		ID:          "act-job-comp",
		Kind:        "test.sync",
		Payload:     []byte(`{}`),
		MaxAttempts: 3,
		AvailableAt: now,
		Now:         now,
	})
	cl, _ := store.ClaimNext(ctx, "w1", []jobs.Kind{"test.sync"}, now)
	_, _ = store.Complete(ctx, cl.ID, cl.LeaseToken, now.Add(-2*time.Hour))

	clearHandler := transporthttp.RequireRole(domain.RoleAdmin)(transporthttp.ActivityClearCompletedHandler(queue, func() time.Time { return now }))
	clearReq := httptest.NewRequest(http.MethodPost, "/api/v1/activity/jobs/clear-completed", nil)
	clearReq = clearReq.WithContext(transporthttp.WithUser(clearReq.Context(), adminUser))
	clearRec := httptest.NewRecorder()
	clearHandler.ServeHTTP(clearRec, clearReq)

	if clearRec.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want 200", clearRec.Code)
	}

	var clearResp struct {
		ClearedCount int64 `json:"cleared_count"`
	}
	if err := json.Unmarshal(clearRec.Body.Bytes(), &clearResp); err != nil {
		t.Fatalf("unmarshal clear: %v", err)
	}
	if clearResp.ClearedCount != 1 {
		t.Errorf("cleared_count = %d, want 1", clearResp.ClearedCount)
	}
}
