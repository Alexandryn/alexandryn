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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

// TestCrossTenant_HandlerToSQL is audit 0016 #135: for every tenant-scoped
// surface (#87 collections, #88 catalog, #104 import candidates), a user
// whose active library is A must not be able to read or mutate a resource
// that lives in library B — traced through the real handler, the real
// repository, and real SQL, not asserted at the repository layer alone.
// A cross-library id is NotFound, with no existence oracle.
func TestCrossTenant_HandlerToSQL(t *testing.T) {
	pool := migratedPool(t)
	ctx := context.Background()

	const libA = domain.DefaultLibraryID // seeded by migration 00009
	const libB = domain.LibraryID("11111111-1111-1111-1111-111111111111")
	xExec(t, pool, "INSERT INTO libraries (id, name, description, allow_reader_uploads, created_at, updated_at) VALUES ($1, 'B', '', false, now(), now())", string(libB))

	// The caller: a reader who is a member of library A only.
	userA := &transporthttp.AuthenticatedUser{
		UserID:    "u-a",
		Username:  "reader-a",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{libA},
	}
	scoped := func(r *http.Request) *http.Request {
		c := transporthttp.WithUser(r.Context(), userA)
		c = transporthttp.WithActiveLibrary(c, libA)
		return r.WithContext(c)
	}

	// --- seed resources that belong to library B -------------------------
	xExec(t, pool, "INSERT INTO works (id, title) VALUES ('w-b', 'B Only Book')")
	xExec(t, pool, "INSERT INTO editions (id, work_id, language, publisher) VALUES ('e-b', 'w-b', 'en', '')")
	xExec(t, pool, "INSERT INTO library_entries (id, edition_id, library_id, added_at) VALUES ('le-b', 'e-b', $1, now())", string(libB))

	collRepo := postgres.NewCollectionRepository(pool)
	cB, _ := domain.NewCollection("coll-b", "B's private shelf")
	if err := collRepo.Save(ctx, libB, cB); err != nil {
		t.Fatalf("seed collection in B: %v", err)
	}

	xExec(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download, library_id)
		VALUES ('src-b', 'B Source', 'local-folder', true, true, true, $1)`, string(libB))
	candRepo := postgres.NewImportCandidateRepository(pool)
	ref, _ := domain.NewFileReference("ref-b", "epub", nil)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := candRepo.Create(ctx, postgres.ImportCandidateRecord{
		ID: "cand-b", SourceID: "src-b", FileReference: ref,
		Status: postgres.ImportCandidateStatusPending, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed candidate in B: %v", err)
	}

	// --- collections ---------------------------------------------------
	t.Run("collections", func(t *testing.T) {
		get := transporthttp.GetCollectionHandler(collRepo)
		rename := transporthttp.RenameCollectionHandler(collRepo)
		del := transporthttp.DeleteCollectionHandler(collRepo)

		cases := []struct {
			name string
			req  *http.Request
			h    http.Handler
		}{
			{"get", pathReq(http.MethodGet, "/api/v1/collections/coll-b", "id", "coll-b", ""), get},
			{"rename", pathReq(http.MethodPatch, "/api/v1/collections/coll-b", "id", "coll-b", `{"name":"hijacked"}`), rename},
			{"delete", pathReq(http.MethodDelete, "/api/v1/collections/coll-b", "id", "coll-b", ""), del},
		}
		for _, tc := range cases {
			rec := httptest.NewRecorder()
			tc.h.ServeHTTP(rec, scoped(tc.req))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s collection from library A: status %d, want 404 (body %s)", tc.name, rec.Code, rec.Body.String())
			}
		}

		// The collection still exists, untouched, in library B.
		got, err := collRepo.FindByID(ctx, libB, "coll-b")
		if err != nil || got.Name() != "B's private shelf" {
			t.Fatalf("collection in B after A's attempts: err=%v name=%q", err, safeCollName(got))
		}
	})

	// --- catalog ------------------------------------------------------
	t.Run("catalog", func(t *testing.T) {
		workRepo := postgres.NewWorkRepository(pool)

		listRec := httptest.NewRecorder()
		transporthttp.LibraryHandler(workRepo).ServeHTTP(listRec, scoped(httptest.NewRequest(http.MethodGet, "/api/v1/library?filter=all", nil)))
		if listRec.Code != http.StatusOK {
			t.Fatalf("GET /library status %d: %s", listRec.Code, listRec.Body.String())
		}
		var page struct {
			Works []struct {
				ID string `json:"id"`
			} `json:"works"`
		}
		_ = json.Unmarshal(listRec.Body.Bytes(), &page)
		for _, wk := range page.Works {
			if wk.ID == "w-b" {
				t.Fatalf("library A's catalog listed library B's work w-b")
			}
		}

		detailReq := pathReq(http.MethodGet, "/api/v1/works/w-b", "id", "w-b", "")
		detailRec := httptest.NewRecorder()
		transporthttp.WorkDetailHandler(workRepo).ServeHTTP(detailRec, scoped(detailReq))
		if detailRec.Code != http.StatusNotFound {
			t.Fatalf("GET /works/w-b from library A: status %d, want 404 (body %s)", detailRec.Code, detailRec.Body.String())
		}
	})

	// --- import candidates ------------------------------------------
	t.Run("import candidates", func(t *testing.T) {
		listRec := httptest.NewRecorder()
		transporthttp.ImportCandidatesListHandler(candRepo).ServeHTTP(listRec, scoped(httptest.NewRequest(http.MethodGet, "/api/v1/import/candidates", nil)))
		if listRec.Code != http.StatusOK {
			t.Fatalf("list candidates status %d: %s", listRec.Code, listRec.Body.String())
		}
		var list struct {
			Candidates []struct {
				ID string `json:"id"`
			} `json:"candidates"`
		}
		_ = json.Unmarshal(listRec.Body.Bytes(), &list)
		if len(list.Candidates) != 0 {
			t.Fatalf("library A listed %d import candidates from library B's source", len(list.Candidates))
		}

		svc := importer.NewService(nil, nil, nil, nil, nil, candRepo, nil, nil, nil)
		rejectReq := pathReq(http.MethodPost, "/api/v1/import/candidates/cand-b/reject", "id", "cand-b", "")
		rejectRec := httptest.NewRecorder()
		transporthttp.ImportCandidateRejectHandler(svc, candRepo).ServeHTTP(rejectRec, scoped(rejectReq))
		if rejectRec.Code != http.StatusNotFound {
			t.Fatalf("reject candidate from library A: status %d, want 404 (body %s)", rejectRec.Code, rejectRec.Body.String())
		}
		if rec, _ := candRepo.Get(ctx, "cand-b"); rec.Status != postgres.ImportCandidateStatusPending {
			t.Fatalf("candidate status changed to %q despite a cross-library reject", rec.Status)
		}
	})
}

func xExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func pathReq(method, target, key, val, body string) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	r.SetPathValue(key, val)
	return r
}

func safeCollName(c *domain.Collection) string {
	if c == nil {
		return "<nil>"
	}
	return c.Name()
}
