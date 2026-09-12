package http

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

type FinishedUserRef struct {
	UserID      string    `json:"user_id"`
	DisplayName string    `json:"display_name"`
	FinishedAt  time.Time `json:"finished_at"`
}

type FinishedWorkItem struct {
	WorkID     string            `json:"work_id"`
	FinishedBy []FinishedUserRef `json:"finished_by"`
}

type finishedWorksResponse struct {
	Works      []FinishedWorkItem `json:"works"`
	NextCursor *string            `json:"nextCursor"`
}

const (
	// finishedWorksDefaultLimit / finishedWorksMaxLimit bound one page of
	// GET /api/v1/library/finished so the response can never be the
	// library's entire completed-reads history.
	finishedWorksDefaultLimit = 50
	finishedWorksMaxLimit     = 200
)

type LeaderboardItem struct {
	WorkID        string `json:"work_id"`
	FinishedCount int    `json:"finished_count"`
}

type leaderboardResponse struct {
	Leaderboard []LeaderboardItem `json:"leaderboard"`
}

// FinishedWorksHandler serves GET /api/v1/library/finished.
// Returns works finished by members (percentage >= 100) in the active library.
// Structurally protects reading privacy.
func FinishedWorksHandler(pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		corrID := CorrelationIDFromContext(ctx)

		if pool == nil {
			WriteError(w, domain.Unavailable, "database not ready", corrID)
			return
		}

		user := UserFromContext(ctx)
		if user == nil {
			WriteError(w, domain.Unauthorized, "missing or invalid authorization token", corrID)
			return
		}

		activeLib := ActiveLibraryFromContext(ctx)
		if activeLib == "" || !libraryInClaims(activeLib, user.Libraries) {
			writeForbidden(w, "you are not a member of that library", corrID)
			return
		}

		limit := finishedWorksDefaultLimit
		if q := r.URL.Query().Get("limit"); q != "" {
			n, err := strconv.Atoi(q)
			if err != nil || n <= 0 {
				WriteError(w, domain.InvalidInput, "invalid limit parameter", corrID)
				return
			}
			limit = min(n, finishedWorksMaxLimit)
		}
		// Keyset cursor: the work_id to resume after. Works are returned in
		// work_id order, so the last work_id of a page is the next cursor.
		cursor := r.URL.Query().Get("cursor")

		start := time.Now()
		// Privacy invariant: ONLY query percentage >= 100. Never project
		// percentage or position. The page CTE picks one page of distinct
		// work ids past the cursor; the outer query then fetches every
		// finisher of just those works.
		query := `
			WITH page AS (
				SELECT work_id
				FROM reading_progress
				WHERE library_id = $1 AND percentage >= 100 AND work_id > $2
				GROUP BY work_id
				ORDER BY work_id
				LIMIT $3
			)
			SELECT
				rp.work_id,
				rp.user_id,
				COALESCE(u.username, '') AS display_name,
				rp.observed_at AS finished_at
			FROM reading_progress rp
			JOIN page ON page.work_id = rp.work_id
			LEFT JOIN users u ON u.id = rp.user_id
			WHERE rp.library_id = $1 AND rp.percentage >= 100
			ORDER BY rp.work_id ASC, rp.observed_at ASC, rp.user_id ASC`

		rows, err := pool.Query(ctx, query, string(activeLib), cursor, limit)
		if err != nil {
			WriteError(w, domain.Internal, "failed to query finished works", corrID)
			return
		}
		defer rows.Close()

		worksMap := make(map[string][]FinishedUserRef)
		var workOrder []string

		for rows.Next() {
			var workID, userID, displayName string
			var finishedAt time.Time

			if err := rows.Scan(&workID, &userID, &displayName, &finishedAt); err != nil {
				WriteError(w, domain.Internal, "failed to read finished works row", corrID)
				return
			}

			if _, exists := worksMap[workID]; !exists {
				workOrder = append(workOrder, workID)
			}
			worksMap[workID] = append(worksMap[workID], FinishedUserRef{
				UserID:      userID,
				DisplayName: displayName,
				FinishedAt:  finishedAt,
			})
		}

		if err := rows.Err(); err != nil {
			WriteError(w, domain.Internal, "failed to read finished works rows", corrID)
			return
		}

		// pgx's pool.Query returns before rows are fetched, so the query
		// time is only known once the scan loop and rows.Err() are done.
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			slog.WarnContext(ctx, "slow finished works query", "query_ms", elapsed.Milliseconds(), "library_id", string(activeLib), "correlation_id", corrID)
		}

		works := make([]FinishedWorkItem, 0, len(workOrder))
		for _, wid := range workOrder {
			works = append(works, FinishedWorkItem{
				WorkID:     wid,
				FinishedBy: worksMap[wid],
			})
		}

		var nextCursor *string
		if len(workOrder) == limit {
			last := workOrder[len(workOrder)-1]
			nextCursor = &last
		}

		writeJSON(w, http.StatusOK, finishedWorksResponse{Works: works, NextCursor: nextCursor}, corrID)
	})
}

// LibraryLeaderboardHandler serves GET /api/v1/library/leaderboard.
// Returns works ranked by completed reads (percentage >= 100) within the active library.
// Structurally protects reading privacy.
func LibraryLeaderboardHandler(pool *pgxpool.Pool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		corrID := CorrelationIDFromContext(ctx)

		if pool == nil {
			WriteError(w, domain.Unavailable, "database not ready", corrID)
			return
		}

		user := UserFromContext(ctx)
		if user == nil {
			WriteError(w, domain.Unauthorized, "missing or invalid authorization token", corrID)
			return
		}

		activeLib := ActiveLibraryFromContext(ctx)
		if activeLib == "" || !libraryInClaims(activeLib, user.Libraries) {
			writeForbidden(w, "you are not a member of that library", corrID)
			return
		}

		limit := 20
		if qLimit := r.URL.Query().Get("limit"); qLimit != "" {
			parsed, err := strconv.Atoi(qLimit)
			if err != nil || parsed <= 0 {
				WriteError(w, domain.InvalidInput, "invalid limit parameter", corrID)
				return
			}
			if parsed > 50 {
				limit = 50
			} else {
				limit = parsed
			}
		}

		start := time.Now()
		// Privacy invariant: ONLY count completed reads (percentage >= 100).
		query := `
			SELECT
				rp.work_id,
				COUNT(DISTINCT rp.user_id) AS finished_count
			FROM reading_progress rp
			WHERE rp.library_id = $1 AND rp.percentage >= 100
			GROUP BY rp.work_id
			ORDER BY finished_count DESC, rp.work_id ASC
			LIMIT $2`

		rows, err := pool.Query(ctx, query, string(activeLib), limit)
		if err != nil {
			WriteError(w, domain.Internal, "failed to query leaderboard", corrID)
			return
		}
		defer rows.Close()

		items := make([]LeaderboardItem, 0)
		for rows.Next() {
			var item LeaderboardItem
			if err := rows.Scan(&item.WorkID, &item.FinishedCount); err != nil {
				WriteError(w, domain.Internal, "failed to read leaderboard row", corrID)
				return
			}
			items = append(items, item)
		}

		if err := rows.Err(); err != nil {
			WriteError(w, domain.Internal, "failed to read leaderboard rows", corrID)
			return
		}

		// pgx returns from Query before rows are fetched — time the scan
		// loop, not the dispatch.
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			slog.WarnContext(ctx, "slow leaderboard query", "query_ms", elapsed.Milliseconds(), "library_id", string(activeLib), "correlation_id", corrID)
		}

		writeJSON(w, http.StatusOK, leaderboardResponse{Leaderboard: items}, corrID)
	})
}

// LazyFinishedWorksHandler wraps FinishedWorksHandler with lazy dependency resolution.
func LazyFinishedWorksHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pool, ok := ref.GetDBPool()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		FinishedWorksHandler(pool).ServeHTTP(w, r)
	})
}

// LazyLibraryLeaderboardHandler wraps LibraryLeaderboardHandler with lazy dependency resolution.
func LazyLibraryLeaderboardHandler(ref *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pool, ok := ref.GetDBPool()
		if !ok {
			WriteError(w, domain.Unavailable, "database not ready", CorrelationIDFromContext(r.Context()))
			return
		}
		LibraryLeaderboardHandler(pool).ServeHTTP(w, r)
	})
}
