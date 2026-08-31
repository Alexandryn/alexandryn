package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

type wireCollectionSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	WorkCount int    `json:"workCount"`
}

type wireCollectionsList struct {
	Collections []wireCollectionSummary `json:"collections"`
}

type wireCollectionDetail struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Works []wireWorkSummary `json:"works"`
}

type wireCollectionCreate struct {
	Name string `json:"name"`
}

type wireCollectionRename struct {
	Name string `json:"name"`
}

type wireCollectionMembershipAdd struct {
	WorkID string `json:"workId"`
}

func mapCollectionDetailToWire(detail *domain.CollectionDetail) wireCollectionDetail {
	works := make([]wireWorkSummary, 0, len(detail.Works))
	for _, w := range detail.Works {
		authors := w.Authors
		if authors == nil {
			authors = []string{}
		}
		colls := make([]wireCollectionRef, 0, len(w.Collections))
		for _, c := range w.Collections {
			colls = append(colls, wireCollectionRef{
				ID:      string(c.ID),
				Name:    c.Name,
				AddedAt: c.AddedAt,
			})
		}
		works = append(works, wireWorkSummary{
			ID:          string(w.ID),
			Title:       w.Title,
			Subtitle:    w.Subtitle,
			Authors:     authors,
			IsOwned:     w.IsOwned,
			Collections: colls,
			AddedAt:     w.AddedAt,
		})
	}
	return wireCollectionDetail{
		ID:    string(detail.ID),
		Name:  detail.Name,
		Works: works,
	}
}

func extractCollectionID(r *http.Request) string {
	id := r.PathValue("id")
	if id != "" {
		return id
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/collections/") {
		trimmed := strings.TrimPrefix(path, "/api/v1/collections/")
		if idx := strings.Index(trimmed, "/"); idx != -1 {
			return trimmed[:idx]
		}
		return trimmed
	}
	return ""
}

func extractCollectionAndWorkID(r *http.Request) (string, string) {
	collID := r.PathValue("id")
	workID := r.PathValue("workId")
	if collID != "" && workID != "" {
		return collID, workID
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/collections/") {
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/collections/"), "/")
		if len(parts) >= 3 && parts[1] == "works" {
			return parts[0], parts[2]
		}
	}
	return collID, workID
}

// ListCollectionsHandler returns the HTTP handler for GET /api/v1/collections (backend-library-api.md FR-6).
func ListCollectionsHandler(repo domain.CollectionRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		colls, err := repo.FindAll(r.Context())
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		wireColls := make([]wireCollectionSummary, 0, len(colls))
		for _, c := range colls {
			wireColls = append(wireColls, wireCollectionSummary{
				ID:        string(c.ID),
				Name:      c.Name,
				WorkCount: c.WorkCount,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(wireCollectionsList{Collections: wireColls})
	})
}

// CreateCollectionHandler returns the HTTP handler for POST /api/v1/collections (backend-library-api.md FR-6).
func CreateCollectionHandler(repo domain.CollectionRepository, ids domain.IDGenerator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		var req wireCollectionCreate
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", id)
			return
		}

		if err := domain.ValidateBoundedText("name", req.Name, 100); err != nil {
			WriteError(w, domain.InvalidInput, err.Error(), id)
			return
		}

		collID := domain.CollectionID(ids.NewID())
		c, err := domain.NewCollection(collID, req.Name)
		if err != nil {
			WriteError(w, domain.InvalidInput, err.Error(), id)
			return
		}

		if err := repo.Save(r.Context(), c); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		resp := wireCollectionDetail{
			ID:    string(c.ID()),
			Name:  c.Name(),
			Works: []wireWorkSummary{},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// GetCollectionHandler returns the HTTP handler for GET /api/v1/collections/{id} (backend-library-api.md FR-6).
func GetCollectionHandler(repo domain.CollectionRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		collID := extractCollectionID(r)
		if err := domain.ValidateBoundedText("id", collID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid collection ID", id)
			return
		}

		detail, err := repo.FindDetail(r.Context(), domain.CollectionID(collID))
		if err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				WriteError(w, domain.NotFound, "no collection with that id", id)
				return
			}
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapCollectionDetailToWire(detail))
	})
}

// RenameCollectionHandler returns the HTTP handler for PATCH /api/v1/collections/{id} (backend-library-api.md FR-6).
func RenameCollectionHandler(repo domain.CollectionRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		collID := extractCollectionID(r)
		if err := domain.ValidateBoundedText("id", collID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid collection ID", id)
			return
		}

		var req wireCollectionRename
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", id)
			return
		}

		if err := domain.ValidateBoundedText("name", req.Name, 100); err != nil {
			WriteError(w, domain.InvalidInput, err.Error(), id)
			return
		}

		if err := repo.Rename(r.Context(), domain.CollectionID(collID), req.Name); err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				WriteError(w, domain.NotFound, "no collection with that id", id)
				return
			}
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		detail, err := repo.FindDetail(r.Context(), domain.CollectionID(collID))
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapCollectionDetailToWire(detail))
	})
}

// DeleteCollectionHandler returns the HTTP handler for DELETE /api/v1/collections/{id} (backend-library-api.md FR-6).
func DeleteCollectionHandler(repo domain.CollectionRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		collID := extractCollectionID(r)
		if err := domain.ValidateBoundedText("id", collID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid collection ID", id)
			return
		}

		if err := repo.Delete(r.Context(), domain.CollectionID(collID)); err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				WriteError(w, domain.NotFound, "no collection with that id", id)
				return
			}
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// AddWorkToCollectionHandler returns the HTTP handler for POST /api/v1/collections/{id}/works (backend-library-api.md FR-7).
func AddWorkToCollectionHandler(repo domain.CollectionRepository, clock func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		collID := extractCollectionID(r)
		if err := domain.ValidateBoundedText("id", collID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid collection ID", id)
			return
		}

		var req wireCollectionMembershipAdd
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", id)
			return
		}

		if req.WorkID == "" {
			WriteError(w, domain.InvalidInput, "workId: missing required field", id)
			return
		}
		if err := domain.ValidateBoundedText("workId", req.WorkID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "workId: not a valid work ID", id)
			return
		}

		now := time.Now().UTC()
		if clock != nil {
			now = clock().UTC()
		}

		if err := repo.AddMember(r.Context(), domain.CollectionID(collID), domain.WorkID(req.WorkID), now); err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				if strings.Contains(err.Error(), "work") {
					WriteError(w, domain.NotFound, "no work with that id", id)
					return
				}
				WriteError(w, domain.NotFound, "no collection with that id", id)
				return
			}
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		detail, err := repo.FindDetail(r.Context(), domain.CollectionID(collID))
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapCollectionDetailToWire(detail))
	})
}

// RemoveWorkFromCollectionHandler returns the HTTP handler for DELETE /api/v1/collections/{id}/works/{workId} (backend-library-api.md FR-7).
func RemoveWorkFromCollectionHandler(repo domain.CollectionRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		collID, workID := extractCollectionAndWorkID(r)
		if err := domain.ValidateBoundedText("id", collID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid collection ID", id)
			return
		}
		if err := domain.ValidateBoundedText("workId", workID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "workId: not a valid work ID", id)
			return
		}

		if err := repo.RemoveMember(r.Context(), domain.CollectionID(collID), domain.WorkID(workID)); err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				if strings.Contains(err.Error(), "membership") {
					WriteError(w, domain.NotFound, "no membership found for that work in this collection", id)
					return
				}
				WriteError(w, domain.NotFound, "no collection with that id", id)
				return
			}
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}



		w.WriteHeader(http.StatusNoContent)
	})
}
