package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

type LibraryWire struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	AllowReaderUploads bool      `json:"allowReaderUploads"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

type LibraryMemberWire struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Username  string    `json:"username,omitempty"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func wireLibrary(l *domain.Library) LibraryWire {
	return LibraryWire{
		ID:                 string(l.ID()),
		Name:               l.Name(),
		Description:        l.Description(),
		AllowReaderUploads: l.AllowReaderUploads(),
		CreatedAt:          l.CreatedAt(),
		UpdatedAt:          l.UpdatedAt(),
	}
}

// ListLibrariesHandler returns all libraries accessible to the authenticated user.
func ListLibrariesHandler(libRepo domain.LibraryRepository, memRepo domain.LibraryMembershipRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		var libs []*domain.Library
		var err error

		if user.Role == domain.RoleAdmin {
			libs, err = libRepo.FindAll(r.Context())
		} else {
			libs, err = libRepo.FindByUser(r.Context(), user.UserID)
		}

		if err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to list libraries", corrID)
			return
		}

		var out []LibraryWire
		for _, l := range libs {
			out = append(out, wireLibrary(l))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"libraries": out})
	})
}

// CreateLibraryHandler creates a new named library (requires Admin).
func CreateLibraryHandler(libRepo domain.LibraryRepository, memRepo domain.LibraryMembershipRepository, idGen domain.IDGenerator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil || user.Role != domain.RoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "Forbidden",
				Message:       "only admins can create libraries",
				CorrelationID: corrID,
			})
			return
		}

		var req struct {
			Name               string `json:"name"`
			Description        string `json:"description"`
			AllowReaderUploads bool   `json:"allowReaderUploads"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request", corrID)
			return
		}

		now := time.Now()
		libID := domain.LibraryID(idGen.NewID())
		lib, err := domain.NewLibrary(libID, req.Name, req.Description, req.AllowReaderUploads, now, now)
		if err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		if err := libRepo.Save(r.Context(), lib); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to save library", corrID)
			return
		}

		// Add creator as admin member
		memID := domain.LibraryMembershipID(idGen.NewID())
		mem, _ := domain.NewLibraryMembership(memID, libID, user.UserID, domain.RoleAdmin, now)
		_ = memRepo.Save(r.Context(), mem)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"library": wireLibrary(lib)})
	})
}

// callerIsLibraryMember reports whether the authenticated user may read
// library libID: a global admin, a library named in the token claims, or
// (claims can lag a just-accepted invitation) a live membership row.
func callerIsLibraryMember(r *http.Request, memRepo domain.LibraryMembershipRepository, libID domain.LibraryID) bool {
	user := UserFromContext(r.Context())
	if user == nil {
		return false
	}
	if user.Role == domain.RoleAdmin || libraryInClaims(libID, user.Libraries) {
		return true
	}
	m, err := memRepo.FindMembership(r.Context(), libID, user.UserID)
	return err == nil && m != nil
}

// callerIsLibraryAdmin reports whether the authenticated user administers
// library libID: a global admin, or a member whose role in that library
// is admin.
func callerIsLibraryAdmin(r *http.Request, memRepo domain.LibraryMembershipRepository, libID domain.LibraryID) bool {
	user := UserFromContext(r.Context())
	if user == nil {
		return false
	}
	if user.Role == domain.RoleAdmin {
		return true
	}
	m, err := memRepo.FindMembership(r.Context(), libID, user.UserID)
	return err == nil && m != nil && m.Role() == domain.RoleAdmin
}

// GetLibraryHandler returns details for one library.
func GetLibraryHandler(libRepo domain.LibraryRepository, memRepo domain.LibraryMembershipRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		id := domain.LibraryID(r.PathValue("id"))

		if !callerIsLibraryMember(r, memRepo, id) {
			// Not a member: 404, not 403 — no "does library X exist" oracle.
			WriteError(w, domain.NotFound, "library not found", corrID)
			return
		}

		lib, err := libRepo.FindByID(r.Context(), id)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "library not found", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"library": wireLibrary(lib)})
	})
}

// UpdateLibraryHandler updates a library's name, description, or ingestion policy.
func UpdateLibraryHandler(libRepo domain.LibraryRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil || user.Role != domain.RoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "Forbidden",
				Message:       "only admins can modify libraries",
				CorrelationID: corrID,
			})
			return
		}

		id := domain.LibraryID(r.PathValue("id"))
		lib, err := libRepo.FindByID(r.Context(), id)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "library not found", corrID)
			return
		}

		var req struct {
			Name               *string `json:"name"`
			Description        *string `json:"description"`
			AllowReaderUploads *bool   `json:"allowReaderUploads"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request", corrID)
			return
		}

		now := time.Now()
		if req.Name != nil {
			if err := lib.Rename(*req.Name, now); err != nil {
				writeDomainError(w, err, corrID)
				return
			}
		}
		if req.Description != nil {
			if err := lib.SetDescription(*req.Description, now); err != nil {
				writeDomainError(w, err, corrID)
				return
			}
		}
		if req.AllowReaderUploads != nil {
			lib.SetAllowReaderUploads(*req.AllowReaderUploads, now)
		}

		if err := libRepo.Save(r.Context(), lib); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to update library", corrID)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"library": wireLibrary(lib)})
	})
}

// DeleteLibraryHandler deletes a custom library.
func DeleteLibraryHandler(libRepo domain.LibraryRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil || user.Role != domain.RoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(errorBody{
				Code:          "Forbidden",
				Message:       "only admins can delete libraries",
				CorrelationID: corrID,
			})
			return
		}

		id := domain.LibraryID(r.PathValue("id"))
		if id == domain.DefaultLibraryID {
			WriteError(w, domain.InvalidInput, "cannot delete default library", corrID)
			return
		}

		if err := libRepo.Delete(r.Context(), id); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to delete library", corrID)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// ListMembersHandler returns all members of a library.
func ListMembersHandler(memRepo domain.LibraryMembershipRepository, userRepo domain.UserRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		id := domain.LibraryID(r.PathValue("id"))

		// The member list (usernames + emails) is admin-only, preventing
		// enumeration of library members by unauthorized users.
		if !callerIsLibraryAdmin(r, memRepo, id) {
			writeForbidden(w, "you must be an admin of that library", corrID)
			return
		}

		mems, err := memRepo.FindByLibrary(r.Context(), id)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to list members", corrID)
			return
		}

		var out []LibraryMemberWire
		for _, m := range mems {
			username := ""
			email := ""
			if u, err := userRepo.FindByID(r.Context(), m.UserID()); err == nil {
				username = u.Username()
				email = u.Email()
			}

			out = append(out, LibraryMemberWire{
				ID:        string(m.ID()),
				UserID:    string(m.UserID()),
				Username:  username,
				Email:     email,
				Role:      string(m.Role()),
				CreatedAt: m.CreatedAt(),
			})
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"members": out})
	})
}

// CreateInvitationHandler generates an invitation token for a library.
func CreateInvitationHandler(invRepo domain.LibraryInvitationRepository, memRepo domain.LibraryMembershipRepository, idGen domain.IDGenerator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		libID := domain.LibraryID(r.PathValue("id"))

		// Only an admin of this library may invite to it — not any
		// global admin, and never a plain reader.
		if !callerIsLibraryAdmin(r, memRepo, libID) {
			writeForbidden(w, "you must be an admin of that library to invite members", corrID)
			return
		}

		var req struct {
			Email string `json:"email"`
			Role  string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request", corrID)
			return
		}

		role, err := domain.ParseRole(req.Role)
		if err != nil {
			role = domain.RoleReader
		}

		rawToken, tokenHash, err := generateRandomToken(32)
		if err != nil {
			WriteError(w, domain.Internal, "failed to generate invite token", corrID)
			return
		}

		now := time.Now()
		invID := domain.LibraryInvitationID(idGen.NewID())
		inv, err := domain.NewLibraryInvitation(invID, libID, req.Email, role, tokenHash, user.UserID, now.Add(7*24*time.Hour), now)
		if err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		if err := invRepo.Save(r.Context(), inv); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to save invitation", corrID)
			return
		}

		inviteURL := fmt.Sprintf("/invite/%s", rawToken)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"invitationToken": rawToken,
			"invitationUrl":   inviteURL,
			"expiresAt":       inv.ExpiresAt(),
		})
	})
}

// AcceptInvitationHandler adds the authenticated user to the invited library.
func AcceptInvitationHandler(invRepo domain.LibraryInvitationRepository, memRepo domain.LibraryMembershipRepository, idGen domain.IDGenerator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := CorrelationIDFromContext(r.Context())
		user := UserFromContext(r.Context())
		if user == nil {
			WriteError(w, domain.Unauthorized, "unauthorized", corrID)
			return
		}

		token := r.PathValue("token")
		if token == "" {
			WriteError(w, domain.InvalidInput, "missing invitation token", corrID)
			return
		}

		h := sha256.Sum256([]byte(token))
		hash := hex.EncodeToString(h[:])

		now := time.Now()
		inv, err := invRepo.FindByTokenHash(r.Context(), hash)
		if err != nil || !inv.IsValid(now) {
			WriteError(w, domain.NotFound, "invalid or expired invitation token", corrID)
			return
		}

		// Add membership
		memID := domain.LibraryMembershipID(idGen.NewID())
		mem, err := domain.NewLibraryMembership(memID, inv.LibraryID(), user.UserID, inv.Role(), now)
		if err != nil {
			writeDomainError(w, err, corrID)
			return
		}

		if err := memRepo.Save(r.Context(), mem); err != nil {
			WriteError(w, domain.CategoryOf(err), "failed to add library membership", corrID)
			return
		}

		inv.MarkUsed(now)
		_ = invRepo.Save(r.Context(), inv)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"libraryId": string(inv.LibraryID()),
			"role":      string(inv.Role()),
		})
	})
}
