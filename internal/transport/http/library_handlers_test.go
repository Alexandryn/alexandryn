package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	transporthttp "github.com/Alexandryn/alexandryn/internal/transport/http"
)

type memInvitations struct {
	byID   map[domain.LibraryInvitationID]*domain.LibraryInvitation
	byHash map[string]*domain.LibraryInvitation
}

func newMemInvitations() *memInvitations {
	return &memInvitations{
		byID:   make(map[domain.LibraryInvitationID]*domain.LibraryInvitation),
		byHash: make(map[string]*domain.LibraryInvitation),
	}
}

func (m *memInvitations) FindByID(_ context.Context, id domain.LibraryInvitationID) (*domain.LibraryInvitation, error) {
	if inv, ok := m.byID[id]; ok {
		return inv, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "invitation not found"}
}

func (m *memInvitations) FindByTokenHash(_ context.Context, hash string) (*domain.LibraryInvitation, error) {
	if inv, ok := m.byHash[hash]; ok {
		return inv, nil
	}
	return nil, &domain.Error{Category: domain.NotFound, Message: "invitation not found"}
}

func (m *memInvitations) FindByLibrary(_ context.Context, libID domain.LibraryID) ([]*domain.LibraryInvitation, error) {
	var out []*domain.LibraryInvitation
	for _, inv := range m.byID {
		if inv.LibraryID() == libID {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (m *memInvitations) Save(_ context.Context, inv *domain.LibraryInvitation) error {
	m.byID[inv.ID()] = inv
	m.byHash[inv.TokenHash()] = inv
	return nil
}

func (m *memInvitations) Delete(_ context.Context, id domain.LibraryInvitationID) error {
	if inv, ok := m.byID[id]; ok {
		delete(m.byHash, inv.TokenHash())
		delete(m.byID, id)
	}
	return nil
}

func TestLibraryHandlers_CRUDAndInvitations(t *testing.T) {
	libs := newMemLibraries()
	mems := newMemMemberships()
	invs := newMemInvitations()
	users := newMemUsers()
	idGen := &seqID{}

	adminUser := &transporthttp.AuthenticatedUser{
		UserID: "u-admin",
		Role:   domain.RoleAdmin,
	}
	readerUser := &transporthttp.AuthenticatedUser{
		UserID: "u-reader",
		Role:   domain.RoleReader,
	}

	uAdmin, _ := domain.NewUser("u-admin", "admin", "admin@alexandryn.local", domain.RoleAdmin, time.Now(), time.Now())
	_ = users.Save(context.Background(), uAdmin)
	uReader, _ := domain.NewUser("u-reader", "reader", "reader@alexandryn.local", domain.RoleReader, time.Now(), time.Now())
	_ = users.Save(context.Background(), uReader)

	// Admin is member of DefaultLibrary
	memAdmin, _ := domain.NewLibraryMembership("mem-1", domain.DefaultLibraryID, "u-admin", domain.RoleAdmin, time.Now())
	_ = mems.Save(context.Background(), memAdmin)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/libraries", transporthttp.ListLibrariesHandler(libs, mems))
	mux.Handle("POST /api/v1/libraries", transporthttp.CreateLibraryHandler(libs, mems, idGen))
	mux.Handle("GET /api/v1/libraries/{id}", transporthttp.GetLibraryHandler(libs, mems))
	mux.Handle("PATCH /api/v1/libraries/{id}", transporthttp.UpdateLibraryHandler(libs))
	mux.Handle("DELETE /api/v1/libraries/{id}", transporthttp.DeleteLibraryHandler(libs))
	mux.Handle("GET /api/v1/libraries/{id}/members", transporthttp.ListMembersHandler(mems, users))
	mux.Handle("POST /api/v1/libraries/{id}/invitations", transporthttp.CreateInvitationHandler(invs, mems, idGen))
	mux.Handle("POST /api/v1/invitations/{token}/accept", transporthttp.AcceptInvitationHandler(invs, mems, idGen))

	var createdLibID string
	var rawInviteToken string

	t.Run("admin creates new library", func(t *testing.T) {
		body := map[string]any{
			"name":               "Sci-Fi & Fantasy",
			"description":        "Speculative fiction",
			"allowReaderUploads": true,
		}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/libraries", bytes.NewReader(buf))
		req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			Library struct {
				ID                 string `json:"id"`
				Name               string `json:"name"`
				AllowReaderUploads bool   `json:"allowReaderUploads"`
			} `json:"library"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.Library.Name != "Sci-Fi & Fantasy" || !res.Library.AllowReaderUploads {
			t.Errorf("unexpected library output: %+v", res.Library)
		}
		createdLibID = res.Library.ID
	})

	t.Run("list libraries returns user libraries", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/libraries", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var res struct {
			Libraries []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"libraries"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if len(res.Libraries) < 2 {
			t.Errorf("expected at least 2 libraries, got %d", len(res.Libraries))
		}
	})

	t.Run("create invitation", func(t *testing.T) {
		body := map[string]string{
			"email": "friend@example.com",
			"role":  "reader",
		}
		buf, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/v1/libraries/"+createdLibID+"/invitations", bytes.NewReader(buf))
		req = req.WithContext(transporthttp.WithUser(req.Context(), adminUser))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			InvitationToken string `json:"invitationToken"`
			InvitationURL   string `json:"invitationUrl"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.InvitationToken == "" {
			t.Fatal("expected non-empty invitation token")
		}
		rawInviteToken = res.InvitationToken
	})

	t.Run("accept invitation adds membership", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/v1/invitations/"+rawInviteToken+"/accept", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), readerUser))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var res struct {
			LibraryID string `json:"libraryId"`
			Role      string `json:"role"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.LibraryID != createdLibID || res.Role != "reader" {
			t.Errorf("unexpected accept output: %+v", res)
		}

		// Verify membership exists
		mem, err := mems.FindMembership(context.Background(), domain.LibraryID(createdLibID), "u-reader")
		if err != nil || mem.Role() != domain.RoleReader {
			t.Fatalf("expected reader membership, got: %+v, err: %v", mem, err)
		}
	})

	// audit 0016 #262: a non-admin non-member must not be able to read a
	// library's details or enumerate its members (usernames + emails).
	t.Run("outsider cannot read library or members", func(t *testing.T) {
		outsider := &transporthttp.AuthenticatedUser{UserID: "u-outsider", Role: domain.RoleReader}
		uOut, _ := domain.NewUser("u-outsider", "outsider", "outsider@x.local", domain.RoleReader, time.Now(), time.Now())
		_ = users.Save(context.Background(), uOut)

		getReq := httptest.NewRequest("GET", "/api/v1/libraries/"+createdLibID, nil)
		getReq = getReq.WithContext(transporthttp.WithUser(getReq.Context(), outsider))
		getRec := httptest.NewRecorder()
		mux.ServeHTTP(getRec, getReq)
		if getRec.Code != http.StatusNotFound {
			t.Fatalf("GET library as outsider = %d, want 404", getRec.Code)
		}

		memReq := httptest.NewRequest("GET", "/api/v1/libraries/"+createdLibID+"/members", nil)
		memReq = memReq.WithContext(transporthttp.WithUser(memReq.Context(), outsider))
		memRec := httptest.NewRecorder()
		mux.ServeHTTP(memRec, memReq)
		if memRec.Code != http.StatusForbidden {
			t.Fatalf("GET members as outsider = %d, want 403", memRec.Code)
		}

		invReq := httptest.NewRequest("POST", "/api/v1/libraries/"+createdLibID+"/invitations",
			bytes.NewReader([]byte(`{"email":"x@y.z","role":"reader"}`)))
		invReq = invReq.WithContext(transporthttp.WithUser(invReq.Context(), outsider))
		invRec := httptest.NewRecorder()
		mux.ServeHTTP(invRec, invReq)
		if invRec.Code != http.StatusForbidden {
			t.Fatalf("POST invitation as outsider = %d, want 403", invRec.Code)
		}
	})

	// The library's own admin (member with admin role, not a global admin)
	// can list members.
	t.Run("library admin can list members", func(t *testing.T) {
		libAdmin := &transporthttp.AuthenticatedUser{UserID: "u-libadmin", Role: domain.RoleReader}
		uLA, _ := domain.NewUser("u-libadmin", "libadmin", "la@x.local", domain.RoleReader, time.Now(), time.Now())
		_ = users.Save(context.Background(), uLA)
		m, _ := domain.NewLibraryMembership("mem-la", domain.LibraryID(createdLibID), "u-libadmin", domain.RoleAdmin, time.Now())
		_ = mems.Save(context.Background(), m)

		req := httptest.NewRequest("GET", "/api/v1/libraries/"+createdLibID+"/members", nil)
		req = req.WithContext(transporthttp.WithUser(req.Context(), libAdmin))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET members as library admin = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	})
}
