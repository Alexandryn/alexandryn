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
	"github.com/Alexandryn/alexandryn/internal/testutil/contracttest"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type mockWorkRepository struct {
	queryLibraryFunc   func(ctx context.Context, q domain.LibraryQuery) (*domain.LibraryPage, error)
	findWorkDetailFunc func(ctx context.Context, id domain.WorkID) (*domain.WorkDetail, error)
}

func (m *mockWorkRepository) FindByID(context.Context, domain.WorkID) (*domain.Work, error) {
	return nil, nil
}
func (m *mockWorkRepository) FindMergedInto(context.Context, domain.WorkID) ([]*domain.Work, error) {
	return nil, nil
}
func (m *mockWorkRepository) Save(context.Context, *domain.Work) error {
	return nil
}
func (m *mockWorkRepository) QueryLibrary(ctx context.Context, q domain.LibraryQuery) (*domain.LibraryPage, error) {
	if m.queryLibraryFunc != nil {
		return m.queryLibraryFunc(ctx, q)
	}
	return &domain.LibraryPage{Works: []*domain.WorkSummary{}}, nil
}
func (m *mockWorkRepository) FindWorkDetail(ctx context.Context, id domain.WorkID, _ domain.LibraryID) (*domain.WorkDetail, error) {
	if m.findWorkDetailFunc != nil {
		return m.findWorkDetailFunc(ctx, id)
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "work not found"}
}

var _ domain.WorkRepository = (*mockWorkRepository)(nil)

// withLibraryUser attaches an authenticated user and active library to a
// request, as AuthMiddleware does in production — the catalog handlers
// resolve both and verify membership.
func withLibraryUser(req *http.Request, lib domain.LibraryID) *http.Request {
	ctx := transporthttp.WithUser(req.Context(), &transporthttp.AuthenticatedUser{
		UserID:    "user-test",
		Username:  "tester",
		Role:      domain.RoleReader,
		Libraries: []domain.LibraryID{lib},
	})
	ctx = transporthttp.WithActiveLibrary(ctx, lib)
	return req.WithContext(ctx)
}

const testLib = domain.DefaultLibraryID

func TestLibraryHandler_Validation(t *testing.T) {
	repo := &mockWorkRepository{}
	handler := transporthttp.LibraryHandler(repo)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCat    domain.Category
	}{
		{
			name:       "valid default query",
			query:      "",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid filter value",
			query:      "?filter=unknown",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "invalid sort value",
			query:      "?sort=relevance",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "limit zero rejected",
			query:      "?limit=0",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "limit negative rejected",
			query:      "?limit=-5",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "limit over 100 rejected",
			query:      "?limit=101",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "limit non-integer rejected",
			query:      "?limit=abc",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "q over 200 chars rejected",
			query:      "?q=" + strings.Repeat("a", 201),
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
		{
			name:       "malformed cursor rejected",
			query:      "?cursor=not-valid-base64-!@#$",
			wantStatus: http.StatusBadRequest,
			wantCat:    domain.InvalidInput,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := withLibraryUser(httptest.NewRequest("GET", "/api/v1/library"+tt.query, nil), testLib)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantCat != "" {
				var errResp struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				}
				if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
					t.Fatalf("decoding error response: %v", err)
				}
				if errResp.Code != string(tt.wantCat) {
					t.Errorf("error code = %q, want %q", errResp.Code, tt.wantCat)
				}
			}
		})
	}
}

func TestLibraryHandler_OpenAPIContract(t *testing.T) {
	validator := contracttest.New(t)
	addedAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	repo := &mockWorkRepository{
		queryLibraryFunc: func(ctx context.Context, q domain.LibraryQuery) (*domain.LibraryPage, error) {
			return &domain.LibraryPage{
				Works: []*domain.WorkSummary{
					{
						ID:       "work-1",
						Title:    "Pride and Prejudice",
						Subtitle: "",
						Authors:  []string{"Jane Austen"},
						IsOwned:  true,
						Collections: []domain.CollectionRef{
							{
								ID:      "coll-1",
								Name:    "Classics",
								AddedAt: &addedAt,
							},
						},
						AddedAt: &addedAt,
					},
				},
				NextCursor: domain.EncodeAddedAtCursor(addedAt, "work-1"),
			}, nil
		},
	}

	handler := transporthttp.LibraryHandler(repo)
	req := withLibraryUser(httptest.NewRequest("GET", "/api/v1/library", nil), testLib)

	rr := validator.ValidateResponse(t, handler, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestWorkDetailHandler_OpenAPIContract(t *testing.T) {
	validator := contracttest.New(t)
	addedAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	pubYear := 1813
	isbn := "9780141439518"
	lang, _ := domain.NewLanguage("en")

	repo := &mockWorkRepository{
		findWorkDetailFunc: func(ctx context.Context, id domain.WorkID) (*domain.WorkDetail, error) {
			if id == "nonexistent" {
				return nil, &domain.Error{Category: domain.NotFound, Message: "work not found"}
			}
			return &domain.WorkDetail{
				ID:               id,
				Title:            "Pride and Prejudice",
				Subtitle:         "",
				Authors:          []string{"Jane Austen"},
				Subjects:         []string{"Classic Literature"},
				OriginalLanguage: &lang,
				OwnedEditions: []domain.OwnedEdition{
					{
						ID:              "ed-1",
						Language:        "en",
						ISBN:            &isbn,
						Publisher:       "T. Egerton",
						PublicationYear: &pubYear,
						AddedAt:         &addedAt,
						Formats:         []string{"epub"},
					},
				},
				Collections: []domain.CollectionRef{
					{
						ID:      "coll-1",
						Name:    "Classics",
						AddedAt: &addedAt,
					},
				},
			}, nil
		},
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/works/{id}", transporthttp.WorkDetailHandler(repo))

	// 1. Success 200
	req := withLibraryUser(httptest.NewRequest("GET", "/api/v1/works/work-1", nil), testLib)
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	// 2. Not found 404
	req404 := withLibraryUser(httptest.NewRequest("GET", "/api/v1/works/nonexistent", nil), testLib)
	rr404 := validator.ValidateResponse(t, mux, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr404.Code)
	}

	// 3. Malformed ID 400
	req400 := withLibraryUser(httptest.NewRequest("GET", "/api/v1/works/"+strings.Repeat("x", 2000), nil), testLib)
	rr400 := validator.ValidateResponse(t, mux, req400)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr400.Code)
	}
}
