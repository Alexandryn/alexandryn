package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// ImportCandidateWireDTO is the JSON response shape for import candidates (backend-import-pipeline.md FR-3, FR-7).
type ImportCandidateWireDTO struct {
	ID                string          `json:"id"`
	SourceID          string          `json:"sourceId"`
	FileReference     FileRefWireDTO  `json:"fileReference"`
	Status            string          `json:"status"`
	ExtractedMetadata json.RawMessage `json:"extractedMetadata,omitempty"`
	MatchCandidates   json.RawMessage `json:"matchCandidates,omitempty"`
	JobID             *string         `json:"jobId,omitempty"`
	LastError         *string         `json:"lastError,omitempty"`
	CreatedAt         string          `json:"createdAt"`
	UpdatedAt         string          `json:"updatedAt"`
}

// FileRefWireDTO represents the wire shape of a file reference in import candidates.
type FileRefWireDTO struct {
	ID        string `json:"id"`
	Format    string `json:"format"`
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
}

// DiscoveryRunner abstracts the discovery coordinator for HTTP handlers.
type DiscoveryRunner interface {
	Discover(ctx context.Context, sourceID string, now time.Time) (importer.DiscoverResult, error)
}

// ImportCandidateLister abstracts candidate retrieval for HTTP handlers.
type ImportCandidateLister interface {
	Get(ctx context.Context, id string) (postgres.ImportCandidateRecord, error)
	List(ctx context.Context, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error)
}

func toCandidateWireDTO(rec postgres.ImportCandidateRecord) ImportCandidateWireDTO {
	var extracted json.RawMessage
	if len(rec.ExtractedMetadata) > 0 {
		extracted = json.RawMessage(rec.ExtractedMetadata)
	}

	var matches json.RawMessage
	if len(rec.MatchCandidates) > 0 {
		matches = json.RawMessage(rec.MatchCandidates)
	}

	return ImportCandidateWireDTO{
		ID:       rec.ID,
		SourceID: rec.SourceID,
		FileReference: FileRefWireDTO{
			ID:        rec.FileReference.ReferenceID,
			Format:    rec.FileReference.Format,
			SizeBytes: rec.FileReference.SizeBytes,
		},
		Status:            rec.Status,
		ExtractedMetadata: extracted,
		MatchCandidates:   matches,
		JobID:             rec.JobID,
		LastError:         rec.LastError,
		CreatedAt:         rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:         rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

// ImportDiscoverHandler handles POST /api/v1/import/discover (FR-1, FR-7).
func ImportDiscoverHandler(runner DiscoveryRunner) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		var body struct {
			SourceID string `json:"sourceId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request body", id)
			return
		}

		sourceID := strings.TrimSpace(body.SourceID)
		if sourceID == "" {
			WriteError(w, domain.InvalidInput, "sourceId is required", id)
			return
		}

		now := time.Now().UTC()
		res, err := runner.Discover(r.Context(), sourceID, now)
		if err != nil {
			cat := domain.CategoryOf(err)
			if cat == domain.NotFound {
				WriteError(w, domain.NotFound, "source not found", id)
				return
			}
			if cat == domain.InvalidInput {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			// Not a domain error at this point — never echo its text
			// (audit 0016 #119); the correlation ID ties it to the log.
			WriteError(w, domain.Internal, "could not start discovery for that source", id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(res)
	})
}

// ImportCandidatesListHandler handles GET /api/v1/import/candidates (FR-7).
func ImportCandidatesListHandler(repo ImportCandidateLister) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceIDParam := r.URL.Query().Get("sourceId")
		statusParam := r.URL.Query().Get("status")

		var sourceIDPtr, statusPtr *string
		if strings.TrimSpace(sourceIDParam) != "" {
			s := strings.TrimSpace(sourceIDParam)
			sourceIDPtr = &s
		}
		if strings.TrimSpace(statusParam) != "" {
			st := strings.TrimSpace(statusParam)
			if !postgres.ValidImportCandidateStatus(st) {
				WriteError(w, domain.InvalidInput, fmt.Sprintf("invalid status filter %q", st), id)
				return
			}
			statusPtr = &st
		}

		list, err := repo.List(r.Context(), sourceIDPtr, statusPtr)
		if err != nil {
			writeDomainError(w, err, id)
			return
		}

		dtos := make([]ImportCandidateWireDTO, 0, len(list))
		for _, rec := range list {
			dtos = append(dtos, toCandidateWireDTO(rec))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": dtos,
		})
	})
}

// ImportConfirmRequestBody is the payload for POST /api/v1/import/candidates/{id}/confirm.
type ImportConfirmRequestBody struct {
	Action             string             `json:"action"`
	EditionID          *string            `json:"editionId,omitempty"`
	OpenLibraryWorkKey *string            `json:"openLibraryWorkKey,omitempty"`
	Metadata           *ExtractedMetaBody `json:"metadata,omitempty"`
}

// ExtractedMetaBody is an optional user-override payload for metadata.
type ExtractedMetaBody struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	ISBN        *string  `json:"isbn,omitempty"`
	Language    *string  `json:"language,omitempty"`
	Publisher   *string  `json:"publisher,omitempty"`
	Description *string  `json:"description,omitempty"`
}

// ImporterService abstracts the import domain operations for HTTP handlers.
type ImporterService interface {
	ConfirmAttachExisting(ctx context.Context, candidateID string, editionID string, now time.Time) error
	ConfirmCreateNew(ctx context.Context, candidateID string, meta extract.ExtractedMetadata, now time.Time) error
	ConfirmOpenLibraryMatch(ctx context.Context, candidateID string, openLibraryWorkKey string, meta extract.ExtractedMetadata, olDetail *openlibrary.DiscoverWorkDetail, now time.Time) error
	Reject(ctx context.Context, candidateID string, now time.Time) error
}

// ImportCandidateConfirmHandler handles POST /api/v1/import/candidates/{id}/confirm (FR-7).
func ImportCandidateConfirmHandler(svc ImporterService, repo ImportCandidateLister, olClient openlibrary.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())
		candidateID := r.PathValue("id")
		if candidateID == "" {
			WriteError(w, domain.InvalidInput, "candidate id is required", id)
			return
		}

		var body ImportConfirmRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			WriteError(w, domain.InvalidInput, "malformed request body", id)
			return
		}

		now := time.Now().UTC()

		switch body.Action {
		case "attach_existing":
			if body.EditionID == nil || strings.TrimSpace(*body.EditionID) == "" {
				WriteError(w, domain.InvalidInput, "editionId is required for attach_existing", id)
				return
			}
			if err := svc.ConfirmAttachExisting(r.Context(), candidateID, strings.TrimSpace(*body.EditionID), now); err != nil {
				writeDomainError(w, err, id)
				return
			}

		case "use_open_library_match":
			if body.OpenLibraryWorkKey == nil || strings.TrimSpace(*body.OpenLibraryWorkKey) == "" {
				WriteError(w, domain.InvalidInput, "openLibraryWorkKey is required for use_open_library_match", id)
				return
			}
			workKey := strings.TrimSpace(*body.OpenLibraryWorkKey)

			cand, err := repo.Get(r.Context(), candidateID)
			if err != nil {
				writeDomainError(w, err, id)
				return
			}

			var meta extract.ExtractedMetadata
			if len(cand.ExtractedMetadata) > 0 {
				_ = json.Unmarshal(cand.ExtractedMetadata, &meta)
			}

			var olDetail *openlibrary.DiscoverWorkDetail
			if olClient != nil {
				detail, err := olClient.GetWork(r.Context(), workKey)
				if err == nil {
					olDetail = detail
				}
			}

			if err := svc.ConfirmOpenLibraryMatch(r.Context(), candidateID, workKey, meta, olDetail, now); err != nil {
				writeDomainError(w, err, id)
				return
			}

		case "create_new":
			cand, err := repo.Get(r.Context(), candidateID)
			if err != nil {
				writeDomainError(w, err, id)
				return
			}

			var meta extract.ExtractedMetadata
			if body.Metadata != nil {
				m, err := extract.NewExtractedMetadata(
					body.Metadata.Title,
					body.Metadata.Authors,
					body.Metadata.ISBN,
					body.Metadata.Language,
					body.Metadata.Publisher,
					body.Metadata.Description,
					nil,
					extract.Format(cand.FileReference.Format),
				)
				if err != nil {
					WriteError(w, domain.InvalidInput, fmt.Sprintf("invalid metadata override: %v", err), id)
					return
				}
				meta = m
			} else if len(cand.ExtractedMetadata) > 0 {
				if err := json.Unmarshal(cand.ExtractedMetadata, &meta); err != nil {
					WriteError(w, domain.Internal, "failed to parse candidate extracted metadata", id)
					return
				}
			} else {
				WriteError(w, domain.InvalidInput, "candidate has no extracted metadata to create work", id)
				return
			}

			if err := svc.ConfirmCreateNew(r.Context(), candidateID, meta, now); err != nil {
				writeDomainError(w, err, id)
				return
			}

		default:
			WriteError(w, domain.InvalidInput, fmt.Sprintf("unknown action %q", body.Action), id)
			return
		}

		updated, err := repo.Get(r.Context(), candidateID)
		if err != nil {
			writeDomainError(w, err, id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(toCandidateWireDTO(updated))
	})
}

// ImportCandidateRejectHandler handles POST /api/v1/import/candidates/{id}/reject (FR-7).
func ImportCandidateRejectHandler(svc ImporterService, repo ImportCandidateLister) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())
		candidateID := r.PathValue("id")
		if candidateID == "" {
			WriteError(w, domain.InvalidInput, "candidate id is required", id)
			return
		}

		now := time.Now().UTC()
		if err := svc.Reject(r.Context(), candidateID, now); err != nil {
			writeDomainError(w, err, id)
			return
		}

		updated, err := repo.Get(r.Context(), candidateID)
		if err != nil {
			writeDomainError(w, err, id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(toCandidateWireDTO(updated))
	})
}
