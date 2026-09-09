package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/testutil/contracttest"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type mockCollectionRepository struct {
	findAllFunc      func(ctx context.Context) ([]*domain.CollectionSummary, error)
	findDetailFunc   func(ctx context.Context, id domain.CollectionID) (*domain.CollectionDetail, error)
	findByIDFunc     func(ctx context.Context, id domain.CollectionID) (*domain.Collection, error)
	saveFunc         func(ctx context.Context, c *domain.Collection) error
	deleteFunc       func(ctx context.Context, id domain.CollectionID) error
	addMemberFunc    func(ctx context.Context, collectionID domain.CollectionID, workID domain.WorkID, addedAt time.Time) error
	removeMemberFunc func(ctx context.Context, collectionID domain.CollectionID, workID domain.WorkID) error
	renameFunc       func(ctx context.Context, id domain.CollectionID, name string) error

	// gotLibraryID records the last library id the handler passed, so a
	// test can assert the active library reaches the repository (#87).
	gotLibraryID domain.LibraryID
}

func (m *mockCollectionRepository) FindByID(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID) (*domain.Collection, error) {
	m.gotLibraryID = libraryID
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return domain.NewCollection(id, "Test Collection")
}

func (m *mockCollectionRepository) Save(ctx context.Context, libraryID domain.LibraryID, c *domain.Collection) error {
	m.gotLibraryID = libraryID
	if m.saveFunc != nil {
		return m.saveFunc(ctx, c)
	}
	return nil
}

func (m *mockCollectionRepository) Delete(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID) error {
	m.gotLibraryID = libraryID
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockCollectionRepository) FindAll(ctx context.Context, libraryID domain.LibraryID) ([]*domain.CollectionSummary, error) {
	m.gotLibraryID = libraryID
	if m.findAllFunc != nil {
		return m.findAllFunc(ctx)
	}
	return []*domain.CollectionSummary{}, nil
}

func (m *mockCollectionRepository) FindDetail(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID) (*domain.CollectionDetail, error) {
	m.gotLibraryID = libraryID
	if m.findDetailFunc != nil {
		return m.findDetailFunc(ctx, id)
	}
	return &domain.CollectionDetail{
		ID:    id,
		Name:  "Test Collection",
		Works: []*domain.WorkSummary{},
	}, nil
}

func (m *mockCollectionRepository) AddMember(ctx context.Context, libraryID domain.LibraryID, collectionID domain.CollectionID, workID domain.WorkID, addedAt time.Time) error {
	m.gotLibraryID = libraryID
	if m.addMemberFunc != nil {
		return m.addMemberFunc(ctx, collectionID, workID, addedAt)
	}
	return nil
}

func (m *mockCollectionRepository) RemoveMember(ctx context.Context, libraryID domain.LibraryID, collectionID domain.CollectionID, workID domain.WorkID) error {
	m.gotLibraryID = libraryID
	if m.removeMemberFunc != nil {
		return m.removeMemberFunc(ctx, collectionID, workID)
	}
	return nil
}

func (m *mockCollectionRepository) Rename(ctx context.Context, libraryID domain.LibraryID, id domain.CollectionID, name string) error {
	m.gotLibraryID = libraryID
	if m.renameFunc != nil {
		return m.renameFunc(ctx, id, name)
	}
	return nil
}

var _ domain.CollectionRepository = (*mockCollectionRepository)(nil)

type mockIDGen struct {
	nextID string
}

func (g *mockIDGen) NewID() string {
	if g.nextID != "" {
		return g.nextID
	}
	return "coll-123"
}

func TestCollectionsHandler_Contract_ListCollections(t *testing.T) {
	validator := contracttest.New(t)

	repo := &mockCollectionRepository{
		findAllFunc: func(ctx context.Context) ([]*domain.CollectionSummary, error) {
			return []*domain.CollectionSummary{
				{ID: "coll-1", Name: "Classics", WorkCount: 5},
				{ID: "coll-2", Name: "Sci-Fi", WorkCount: 2},
			}, nil
		},
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/collections", transporthttp.ListCollectionsHandler(repo))

	req := httptest.NewRequest("GET", "/api/v1/collections", nil)
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}
}

func TestCollectionsHandler_Contract_CreateCollection(t *testing.T) {
	validator := contracttest.New(t)

	repo := &mockCollectionRepository{}
	ids := &mockIDGen{nextID: "01JXXXXXXXXXXXXXXXXXXXXXXZ"}

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/collections", transporthttp.CreateCollectionHandler(repo, ids))

	// 1. Success 201
	body := `{"name":"Classics"}`
	req := httptest.NewRequest("POST", "/api/v1/collections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rr.Code, rr.Body.String())
	}

	// 2. Validation error 400 (empty name)
	badBody := `{"name":""}`
	badReq := httptest.NewRequest("POST", "/api/v1/collections", strings.NewReader(badBody))
	badReq.Header.Set("Content-Type", "application/json")
	rr400 := validator.ValidateResponse(t, mux, badReq)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rr400.Code, rr400.Body.String())
	}

	// 3. Validation error 400 (name too long)
	tooLongBody := `{"name":"` + strings.Repeat("x", 101) + `"}`
	tooLongReq := httptest.NewRequest("POST", "/api/v1/collections", strings.NewReader(tooLongBody))
	tooLongReq.Header.Set("Content-Type", "application/json")
	rrTooLong := validator.ValidateResponse(t, mux, tooLongReq)
	if rrTooLong.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", rrTooLong.Code, rrTooLong.Body.String())
	}
}

func TestCollectionsHandler_Contract_GetCollection(t *testing.T) {
	validator := contracttest.New(t)

	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	repo := &mockCollectionRepository{
		findDetailFunc: func(ctx context.Context, id domain.CollectionID) (*domain.CollectionDetail, error) {
			if id == "nonexistent" {
				return nil, &domain.Error{Category: domain.NotFound, Message: "no collection with that id"}
			}
			return &domain.CollectionDetail{
				ID:   id,
				Name: "Classics",
				Works: []*domain.WorkSummary{
					{
						ID:          "work-1",
						Title:       "Middlemarch",
						Subtitle:    "",
						Authors:     []string{"George Eliot"},
						IsOwned:     true,
						Collections: []domain.CollectionRef{{ID: id, Name: "Classics", AddedAt: &now}},
						AddedAt:     &now,
					},
				},
			}, nil
		},
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/collections/{id}", transporthttp.GetCollectionHandler(repo))

	// 1. Success 200
	req := httptest.NewRequest("GET", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ", nil)
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	// 2. Not found 404
	req404 := httptest.NewRequest("GET", "/api/v1/collections/nonexistent", nil)
	rr404 := validator.ValidateResponse(t, mux, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr404.Code)
	}

	// 3. Malformed ID 400
	req400 := httptest.NewRequest("GET", "/api/v1/collections/"+strings.Repeat("x", 2000), nil)
	rr400 := validator.ValidateResponse(t, mux, req400)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr400.Code)
	}
}

func TestCollectionsHandler_Contract_RenameCollection(t *testing.T) {
	validator := contracttest.New(t)

	repo := &mockCollectionRepository{
		renameFunc: func(ctx context.Context, id domain.CollectionID, name string) error {
			if id == "nonexistent" {
				return &domain.Error{Category: domain.NotFound, Message: "no collection with that id"}
			}
			return nil
		},
		findDetailFunc: func(ctx context.Context, id domain.CollectionID) (*domain.CollectionDetail, error) {
			if id == "nonexistent" {
				return nil, &domain.Error{Category: domain.NotFound, Message: "no collection with that id"}
			}
			return &domain.CollectionDetail{
				ID:    id,
				Name:  "Renamed Collection",
				Works: []*domain.WorkSummary{},
			}, nil
		},
	}

	mux := http.NewServeMux()
	mux.Handle("PATCH /api/v1/collections/{id}", transporthttp.RenameCollectionHandler(repo))

	// 1. Success 200
	body := `{"name":"Renamed Collection"}`
	req := httptest.NewRequest("PATCH", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	// 2. Not found 404
	req404 := httptest.NewRequest("PATCH", "/api/v1/collections/nonexistent", strings.NewReader(body))
	req404.Header.Set("Content-Type", "application/json")
	rr404 := validator.ValidateResponse(t, mux, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr404.Code)
	}

	// 3. Invalid input 400 (control character)
	badBody := "{\"name\":\"Bad\\x00Name\"}"
	req400 := httptest.NewRequest("PATCH", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ", strings.NewReader(badBody))
	req400.Header.Set("Content-Type", "application/json")
	rr400 := validator.ValidateResponse(t, mux, req400)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr400.Code)
	}
}

func TestCollectionsHandler_Contract_DeleteCollection(t *testing.T) {
	validator := contracttest.New(t)

	repo := &mockCollectionRepository{
		deleteFunc: func(ctx context.Context, id domain.CollectionID) error {
			if id == "nonexistent" {
				return &domain.Error{Category: domain.NotFound, Message: "no collection with that id"}
			}
			return nil
		},
	}

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/collections/{id}", transporthttp.DeleteCollectionHandler(repo))

	// 1. Success 204
	req := httptest.NewRequest("DELETE", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ", nil)
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}

	// 2. Not found 404
	req404 := httptest.NewRequest("DELETE", "/api/v1/collections/nonexistent", nil)
	rr404 := validator.ValidateResponse(t, mux, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr404.Code)
	}

	// 3. Malformed ID 400
	req400 := httptest.NewRequest("DELETE", "/api/v1/collections/"+strings.Repeat("x", 2000), nil)
	rr400 := validator.ValidateResponse(t, mux, req400)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr400.Code)
	}
}

func TestCollectionsHandler_Contract_AddWorkToCollection(t *testing.T) {
	validator := contracttest.New(t)

	repo := &mockCollectionRepository{
		addMemberFunc: func(ctx context.Context, collectionID domain.CollectionID, workID domain.WorkID, addedAt time.Time) error {
			if collectionID == "nonexistent" {
				return &domain.Error{Category: domain.NotFound, Message: "collection not found"}
			}
			if workID == "nonexistent-work" {
				return &domain.Error{Category: domain.NotFound, Message: "work not found"}
			}
			return nil
		},
		findDetailFunc: func(ctx context.Context, id domain.CollectionID) (*domain.CollectionDetail, error) {
			if id == "nonexistent" {
				return nil, &domain.Error{Category: domain.NotFound, Message: "collection not found"}
			}
			return &domain.CollectionDetail{
				ID:    id,
				Name:  "Classics",
				Works: []*domain.WorkSummary{},
			}, nil
		},
	}

	fixedTime := time.Date(2026, 1, 20, 8, 0, 0, 0, time.UTC)
	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/collections/{id}/works", transporthttp.AddWorkToCollectionHandler(repo, func() time.Time { return fixedTime }))

	// 1. Success 200
	body := `{"workId":"01JXXXXXXXXXXXXXXXXXXXXXXX"}`
	req := httptest.NewRequest("POST", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ/works", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
	}

	// 2. Not found 404 (collection)
	req404 := httptest.NewRequest("POST", "/api/v1/collections/nonexistent/works", strings.NewReader(body))
	req404.Header.Set("Content-Type", "application/json")
	rr404 := validator.ValidateResponse(t, mux, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr404.Code)
	}

	// 3. Not found 404 (work)
	badWorkBody := `{"workId":"nonexistent-work"}`
	reqWork404 := httptest.NewRequest("POST", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ/works", strings.NewReader(badWorkBody))
	reqWork404.Header.Set("Content-Type", "application/json")
	rrWork404 := validator.ValidateResponse(t, mux, reqWork404)
	if rrWork404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rrWork404.Code)
	}

	// 4. Missing workId 400
	missingBody := `{}`
	req400 := httptest.NewRequest("POST", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ/works", strings.NewReader(missingBody))
	req400.Header.Set("Content-Type", "application/json")
	rr400 := validator.ValidateResponse(t, mux, req400)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr400.Code)
	}
}

func TestCollectionsHandler_Contract_RemoveWorkFromCollection(t *testing.T) {
	validator := contracttest.New(t)

	repo := &mockCollectionRepository{
		removeMemberFunc: func(ctx context.Context, collectionID domain.CollectionID, workID domain.WorkID) error {
			if collectionID == "nonexistent" {
				return &domain.Error{Category: domain.NotFound, Message: "no collection with that id"}
			}
			if workID == "nonexistent-work" {
				return &domain.Error{Category: domain.NotFound, Message: "no membership found for that work in this collection"}
			}
			return nil
		},
	}

	mux := http.NewServeMux()
	mux.Handle("DELETE /api/v1/collections/{id}/works/{workId}", transporthttp.RemoveWorkFromCollectionHandler(repo))

	// 1. Success 204
	req := httptest.NewRequest("DELETE", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ/works/01JXXXXXXXXXXXXXXXXXXXXXXX", nil)
	rr := validator.ValidateResponse(t, mux, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}

	// 2. Not found 404 (membership not found)
	req404 := httptest.NewRequest("DELETE", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ/works/nonexistent-work", nil)
	rr404 := validator.ValidateResponse(t, mux, req404)
	if rr404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr404.Code)
	}
	if !strings.Contains(rr404.Body.String(), "no membership found for that work in this collection") {
		t.Errorf("body = %s, want to contain 'no membership found for that work in this collection'", rr404.Body.String())
	}

	// 3. Not found 404 (collection not found)
	reqColl404 := httptest.NewRequest("DELETE", "/api/v1/collections/nonexistent/works/01JXXXXXXXXXXXXXXXXXXXXXXX", nil)
	rrColl404 := validator.ValidateResponse(t, mux, reqColl404)
	if rrColl404.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rrColl404.Code)
	}
	if !strings.Contains(rrColl404.Body.String(), "no collection with that id") {
		t.Errorf("body = %s, want to contain 'no collection with that id'", rrColl404.Body.String())
	}

	// 4. Malformed workId 400
	req400 := httptest.NewRequest("DELETE", "/api/v1/collections/01JXXXXXXXXXXXXXXXXXXXXXXZ/works/"+strings.Repeat("x", 2000), nil)
	rr400 := validator.ValidateResponse(t, mux, req400)
	if rr400.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr400.Code)
	}
}

// #87: every collection handler must scope its repository call to the
// request's active library, so a caller in one library cannot read or
// mutate another library's collections.
func TestCollectionsHandler_ScopesToActiveLibrary(t *testing.T) {
	const activeLib = domain.LibraryID("lib-xyz")

	cases := map[string]func(repo *mockCollectionRepository) (http.Handler, *http.Request){
		"list": func(repo *mockCollectionRepository) (http.Handler, *http.Request) {
			return transporthttp.ListCollectionsHandler(repo), httptest.NewRequest("GET", "/api/v1/collections", nil)
		},
		"get": func(repo *mockCollectionRepository) (http.Handler, *http.Request) {
			r := httptest.NewRequest("GET", "/api/v1/collections/c1", nil)
			r.SetPathValue("id", "c1")
			return transporthttp.GetCollectionHandler(repo), r
		},
		"rename": func(repo *mockCollectionRepository) (http.Handler, *http.Request) {
			r := httptest.NewRequest("PATCH", "/api/v1/collections/c1", strings.NewReader(`{"name":"x"}`))
			r.SetPathValue("id", "c1")
			return transporthttp.RenameCollectionHandler(repo), r
		},
		"delete": func(repo *mockCollectionRepository) (http.Handler, *http.Request) {
			r := httptest.NewRequest("DELETE", "/api/v1/collections/c1", nil)
			r.SetPathValue("id", "c1")
			return transporthttp.DeleteCollectionHandler(repo), r
		},
		"create": func(repo *mockCollectionRepository) (http.Handler, *http.Request) {
			return transporthttp.CreateCollectionHandler(repo, &mockIDGen{nextID: "c-new"}),
				httptest.NewRequest("POST", "/api/v1/collections", strings.NewReader(`{"name":"x"}`))
		},
	}

	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &mockCollectionRepository{}
			h, req := build(repo)
			req = req.WithContext(transporthttp.WithActiveLibrary(req.Context(), activeLib))
			h.ServeHTTP(httptest.NewRecorder(), req)
			if repo.gotLibraryID != activeLib {
				t.Fatalf("%s: repo received library %q, want %q", name, repo.gotLibraryID, activeLib)
			}
		})
	}
}
