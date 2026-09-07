package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

func TestLeaderboard_NilPoolReturnsUnavailable(t *testing.T) {
	finishedHandler := transporthttp.FinishedWorksHandler(nil)
	leaderboardHandler := transporthttp.LibraryLeaderboardHandler(nil)

	user := &transporthttp.AuthenticatedUser{
		UserID:    domain.UserID("user-1"),
		Username:  "u1",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{domain.DefaultLibraryID},
	}

	reqF := httptest.NewRequest(http.MethodGet, "/api/v1/library/finished", nil)
	reqF = reqF.WithContext(transporthttp.WithUser(reqF.Context(), user))
	recF := httptest.NewRecorder()
	finishedHandler.ServeHTTP(recF, reqF)

	if recF.Code != http.StatusServiceUnavailable {
		t.Errorf("finished with nil pool: status = %d, want 503", recF.Code)
	}

	reqL := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard", nil)
	reqL = reqL.WithContext(transporthttp.WithUser(reqL.Context(), user))
	recL := httptest.NewRecorder()
	leaderboardHandler.ServeHTTP(recL, reqL)

	if recL.Code != http.StatusServiceUnavailable {
		t.Errorf("leaderboard with nil pool: status = %d, want 503", recL.Code)
	}
}

func TestLazyLeaderboardHandlers_DatabaseNotReady(t *testing.T) {
	poolRef := &transporthttp.PoolRef{}
	lazyFinished := transporthttp.LazyFinishedWorksHandler(poolRef)
	lazyLeaderboard := transporthttp.LazyLibraryLeaderboardHandler(poolRef)

	reqF := httptest.NewRequest(http.MethodGet, "/api/v1/library/finished", nil)
	recF := httptest.NewRecorder()
	lazyFinished.ServeHTTP(recF, reqF)

	if recF.Code != http.StatusServiceUnavailable {
		t.Errorf("lazy finished without db: status = %d, want 503", recF.Code)
	}

	reqL := httptest.NewRequest(http.MethodGet, "/api/v1/library/leaderboard", nil)
	recL := httptest.NewRecorder()
	lazyLeaderboard.ServeHTTP(recL, reqL)

	if recL.Code != http.StatusServiceUnavailable {
		t.Errorf("lazy leaderboard without db: status = %d, want 503", recL.Code)
	}
}
