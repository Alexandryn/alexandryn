//go:build integration

package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// scanResponseForPrivacyViolations checks that response JSON contains zero reading coordinates or progress keys.
func scanResponseForPrivacyViolations(t *testing.T, body []byte) {
	t.Helper()
	s := strings.ToLower(string(body))
	prohibited := []string{
		"percentage", "position", "cfi", "chapter", "location", "progress",
	}
	for _, p := range prohibited {
		if strings.Contains(s, `"`+p+`"`) {
			t.Errorf("privacy violation: response body contains prohibited key %q: %s", p, string(body))
		}
	}
}

func TestLeaderboard_PrivacyAndTenantIsolation(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	libA := "00000000-0000-0000-0000-000000000001"
	libB := "00000000-0000-0000-0000-000000000002"

	// 1. Seed libraries
	if _, err := pool.Exec(ctx, `INSERT INTO libraries (id, name, created_at, updated_at) VALUES ($1, 'Lib A', now(), now()) ON CONFLICT DO NOTHING`, libA); err != nil {
		t.Fatalf("seed lib A: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO libraries (id, name, created_at, updated_at) VALUES ($1, 'Lib B', now(), now()) ON CONFLICT DO NOTHING`, libB); err != nil {
		t.Fatalf("seed lib B: %v", err)
	}

	// 2. Seed users
	userAlice := "user-alice"
	userBob := "user-bob"
	userCharlie := "user-charlie"
	userDave := "user-dave"

	for _, u := range []struct {
		id, name, email string
	}{
		{userAlice, "alice", "alice@example.com"},
		{userBob, "bob", "bob@example.com"},
		{userCharlie, "charlie", "charlie@example.com"},
		{userDave, "dave", "dave@example.com"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, username, email, role, created_at, updated_at)
			VALUES ($1, $2, $3, 'reader', now(), now()) ON CONFLICT DO NOTHING`, u.id, u.name, u.email); err != nil {
			t.Fatalf("seed user %s: %v", u.id, err)
		}
	}

	// 3. Seed works
	work1 := "work-00000000-0001"
	work2 := "work-00000000-0002"
	for _, w := range []string{work1, work2} {
		if _, err := pool.Exec(ctx, `INSERT INTO works (id, title)
			VALUES ($1, 'Book ' || $1) ON CONFLICT DO NOTHING`, w); err != nil {
			t.Fatalf("seed work %s: %v", w, err)
		}
	}

	// 4. Seed reading_progress rows:
	// - Alice: finished Work 1 (percentage = 100) in Lib A
	// - Bob: IN PROGRESS on Work 1 (percentage = 99.9) in Lib A — MUST BE EXCLUDED!
	// - Charlie: finished Work 1 (percentage = 100) and Work 2 (percentage = 105) in Lib A
	// - Dave: finished Work 1 (percentage = 100) in Lib B — TENANT ISOLATION (must not appear in Lib A)!
	now := time.Now().UTC()
	progressData := []struct {
		id, workID, userID, libID string
		pct                       float64
	}{
		{"rp-alice-w1", work1, userAlice, libA, 100.0},
		{"rp-bob-w1", work1, userBob, libA, 99.9}, // Sensitive in-progress reading!
		{"rp-charlie-w1", work1, userCharlie, libA, 100.0},
		{"rp-charlie-w2", work2, userCharlie, libA, 105.0},
		{"rp-dave-w1", work1, userDave, libB, 100.0}, // Lib B
	}

	for _, p := range progressData {
		_, err := pool.Exec(ctx, `INSERT INTO reading_progress
			(id, work_id, percentage, device_id, observed_at, user_id, library_id, epoch)
			VALUES ($1, $2, $3, 'dev-1', $4, $5, $6, 1)
			ON CONFLICT (id) DO UPDATE SET percentage = EXCLUDED.percentage`,
			p.id, p.workID, p.pct, now, p.userID, p.libID)
		if err != nil {
			t.Fatalf("seed progress %s: %v", p.id, err)
		}
	}

	finishedHandler := transporthttp.FinishedWorksHandler(pool)
	leaderboardHandler := transporthttp.LibraryLeaderboardHandler(pool)

	aliceUser := &transporthttp.AuthenticatedUser{
		UserID:    domain.UserID(userAlice),
		Username:  "alice",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{domain.LibraryID(libA)},
	}

	// --- Test GET /api/v1/library/finished ---
	t.Run("GET /api/v1/library/finished privacy and tenant isolation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/library/finished", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), aliceUser))
		req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), domain.LibraryID(libA)))
		rec := httptest.NewRecorder()

		finishedHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("finished status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
		}

		bodyBytes := rec.Body.Bytes()
		scanResponseForPrivacyViolations(t, bodyBytes)

		var resp struct {
			Works []struct {
				WorkID     string `json:"work_id"`
				FinishedBy []struct {
					UserID      string `json:"user_id"`
					DisplayName string `json:"display_name"`
					FinishedAt  string `json:"finished_at"`
				} `json:"finished_by"`
			} `json:"works"`
		}
		if err := json.Unmarshal(bodyBytes, &resp); err != nil {
			t.Fatalf("unmarshal finished works: %v", err)
		}

		// Find work 1
		var w1Completions []string
		for _, w := range resp.Works {
			if w.WorkID == work1 {
				for _, fb := range w.FinishedBy {
					w1Completions = append(w1Completions, fb.UserID)
				}
			}
		}

		// 1. Bob (percentage 99.9) MUST NOT be present
		for _, uid := range w1Completions {
			if uid == userBob {
				t.Errorf("PRIVACY VIOLATION: userBob (percentage 99.9) appeared in finished list")
			}
			if uid == userDave {
				t.Errorf("TENANT ISOLATION VIOLATION: userDave from Lib B appeared in Lib A finished list")
			}
		}

		// 2. Alice and Charlie MUST be present
		hasAlice := false
		hasCharlie := false
		for _, uid := range w1Completions {
			if uid == userAlice {
				hasAlice = true
			}
			if uid == userCharlie {
				hasCharlie = true
			}
		}
		if !hasAlice {
			t.Errorf("expected Alice in finished list for work 1")
		}
		if !hasCharlie {
			t.Errorf("expected Charlie in finished list for work 1")
		}
	})

	// --- Test GET /api/v1/library/leaderboard ---
	t.Run("GET /api/v1/library/leaderboard count excludes partial readers and isolates tenants", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), aliceUser))
		req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), domain.LibraryID(libA)))
		rec := httptest.NewRecorder()

		leaderboardHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("leaderboard status = %d, want 200. Body: %s", rec.Code, rec.Body.String())
		}

		bodyBytes := rec.Body.Bytes()
		scanResponseForPrivacyViolations(t, bodyBytes)

		var resp struct {
			Leaderboard []struct {
				WorkID        string `json:"work_id"`
				FinishedCount int    `json:"finished_count"`
			} `json:"leaderboard"`
		}
		if err := json.Unmarshal(bodyBytes, &resp); err != nil {
			t.Fatalf("unmarshal leaderboard: %v", err)
		}

		if len(resp.Leaderboard) == 0 {
			t.Fatalf("expected at least 1 leaderboard entry")
		}

		// Work 1 should have finished_count = 2 (Alice & Charlie only; NOT Bob at 99.9, NOT Dave in Lib B)
		for _, item := range resp.Leaderboard {
			if item.WorkID == work1 {
				if item.FinishedCount != 2 {
					t.Errorf("work 1 finished_count = %d, want 2 (Alice + Charlie)", item.FinishedCount)
				}
			}
		}
	})

	// --- T3.4 / FR-6: IDOR and authorization tests ---
	t.Run("IDOR: non-member of library receives 403 Forbidden", func(t *testing.T) {
		daveUser := &transporthttp.AuthenticatedUser{
			UserID:    domain.UserID(userDave),
			Username:  "dave",
			Role:      domain.RoleReader,
			Libraries: []domain.LibraryID{domain.LibraryID(libB)}, // Only Lib B
		}

		// Dave tries to query Lib A finished works
		reqFinished := httptest.NewRequest(http.MethodGet, "/api/v1/library/finished", nil)
		reqFinished = reqFinished.WithContext(transporthttp.WithUser(reqFinished.Context(), daveUser))
		reqFinished = reqFinished.WithContext(transporthttp.WithActiveLibrary(reqFinished.Context(), domain.LibraryID(libA)))
		recFinished := httptest.NewRecorder()
		finishedHandler.ServeHTTP(recFinished, reqFinished)

		if recFinished.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for cross-library access on finished works, got %d", recFinished.Code)
		}

		// Dave tries to query Lib A leaderboard
		reqLeaderboard := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard", nil)
		reqLeaderboard = reqLeaderboard.WithContext(transporthttp.WithUser(reqLeaderboard.Context(), daveUser))
		reqLeaderboard = reqLeaderboard.WithContext(transporthttp.WithActiveLibrary(reqLeaderboard.Context(), domain.LibraryID(libA)))
		recLeaderboard := httptest.NewRecorder()
		leaderboardHandler.ServeHTTP(recLeaderboard, reqLeaderboard)

		if recLeaderboard.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for cross-library access on leaderboard, got %d", recLeaderboard.Code)
		}
	})

	t.Run("Unauthenticated requests receive 401 Unauthorized", func(t *testing.T) {
		reqFinished := httptest.NewRequest(http.MethodGet, "/api/v1/library/finished", nil)
		reqFinished = reqFinished.WithContext(transporthttp.WithActiveLibrary(reqFinished.Context(), domain.LibraryID(libA)))
		recFinished := httptest.NewRecorder()
		finishedHandler.ServeHTTP(recFinished, reqFinished)

		if recFinished.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for unauthenticated finished works, got %d", recFinished.Code)
		}

		reqLeaderboard := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard", nil)
		reqLeaderboard = reqLeaderboard.WithContext(transporthttp.WithActiveLibrary(reqLeaderboard.Context(), domain.LibraryID(libA)))
		recLeaderboard := httptest.NewRecorder()
		leaderboardHandler.ServeHTTP(recLeaderboard, reqLeaderboard)

		if recLeaderboard.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for unauthenticated leaderboard, got %d", recLeaderboard.Code)
		}
	})

	t.Run("Limit query parameter validation", func(t *testing.T) {
		// Valid limit = 1
		req := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard?limit=1", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), aliceUser))
		req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), domain.LibraryID(libA)))
		rec := httptest.NewRecorder()
		leaderboardHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var resp struct {
			Leaderboard []struct {
				WorkID string `json:"work_id"`
			} `json:"leaderboard"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(resp.Leaderboard) != 1 {
			t.Errorf("leaderboard items = %d, want 1 when limit=1", len(resp.Leaderboard))
		}

		// Invalid negative limit -> 400
		reqBad := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard?limit=-5", nil)
		reqBad = reqBad.WithContext(transporthttp.WithUser(reqBad.Context(), aliceUser))
		reqBad = reqBad.WithContext(transporthttp.WithActiveLibrary(reqBad.Context(), domain.LibraryID(libA)))
		recBad := httptest.NewRecorder()
		leaderboardHandler.ServeHTTP(recBad, reqBad)

		if recBad.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 for negative limit", recBad.Code)
		}

		// Non-numeric limit -> 400
		reqNonNum := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard?limit=foo", nil)
		reqNonNum = reqNonNum.WithContext(transporthttp.WithUser(reqNonNum.Context(), aliceUser))
		reqNonNum = reqNonNum.WithContext(transporthttp.WithActiveLibrary(reqNonNum.Context(), domain.LibraryID(libA)))
		recNonNum := httptest.NewRecorder()
		leaderboardHandler.ServeHTTP(recNonNum, reqNonNum)

		if recNonNum.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400 for non-numeric limit", recNonNum.Code)
		}
	})

	t.Run("Empty library returns 200 with empty arrays", func(t *testing.T) {
		libEmpty := "00000000-0000-0000-0000-000000000099"
		if _, err := pool.Exec(ctx, `INSERT INTO libraries (id, name, created_at, updated_at) VALUES ($1, 'Empty Lib', now(), now()) ON CONFLICT DO NOTHING`, libEmpty); err != nil {
			t.Fatalf("seed empty lib: %v", err)
		}

		emptyUser := &transporthttp.AuthenticatedUser{
			UserID:    domain.UserID("user-empty"),
			Username:  "empty",
			Role:      domain.RoleReader,
			Libraries: []domain.LibraryID{domain.LibraryID(libEmpty)},
		}

		reqF := httptest.NewRequest(http.MethodGet, "/api/v1/library/finished", nil)
		reqF = reqF.WithContext(transporthttp.WithUser(reqF.Context(), emptyUser))
		reqF = reqF.WithContext(transporthttp.WithActiveLibrary(reqF.Context(), domain.LibraryID(libEmpty)))
		recF := httptest.NewRecorder()
		finishedHandler.ServeHTTP(recF, reqF)

		if recF.Code != http.StatusOK {
			t.Fatalf("finished status = %d, want 200", recF.Code)
		}
		if !strings.Contains(recF.Body.String(), `"works":[]`) {
			t.Errorf("expected empty works array in body: %s", recF.Body.String())
		}

		reqL := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard", nil)
		reqL = reqL.WithContext(transporthttp.WithUser(reqL.Context(), emptyUser))
		reqL = reqL.WithContext(transporthttp.WithActiveLibrary(reqL.Context(), domain.LibraryID(libEmpty)))
		recL := httptest.NewRecorder()
		leaderboardHandler.ServeHTTP(recL, reqL)

		if recL.Code != http.StatusOK {
			t.Fatalf("leaderboard status = %d, want 200", recL.Code)
		}
		if !strings.Contains(recL.Body.String(), `"leaderboard":[]`) {
			t.Errorf("expected empty leaderboard array in body: %s", recL.Body.String())
		}
	})
}
