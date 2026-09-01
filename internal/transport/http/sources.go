package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Alexandryn/alexandryn/internal/adapters/sources"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources/local"
	"github.com/Alexandryn/alexandryn/internal/adapters/sources/opds"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// SourceRecordRepository defines the repository methods required by the source HTTP handlers.
type SourceRecordRepository interface {
	Create(ctx context.Context, rec postgres.SourceRecord) error
	Get(ctx context.Context, id string) (postgres.SourceRecord, error)
	List(ctx context.Context) ([]postgres.SourceRecord, error)
	UpdateConfig(ctx context.Context, id, label, basePath, baseURL string) error
	SetCredential(ctx context.Context, id string, ciphertext, nonce []byte) error
	UpdateHealth(ctx context.Context, id, status, detail string, checkedAt time.Time, caps domain.SourceCapabilities, searchLinkURL string) error
}

type wireSourceConfig struct {
	BasePath *string `json:"basePath,omitempty"`
	BaseURL  *string `json:"baseUrl,omitempty"`
}

type wireSourceHealth struct {
	Status    string     `json:"status"`
	CheckedAt *time.Time `json:"checkedAt"`
	Detail    *string    `json:"detail"`
}

type wireSourceCapabilities struct {
	CanList     bool `json:"canList"`
	CanSearch   bool `json:"canSearch"`
	CanDownload bool `json:"canDownload"`
}

type wireSource struct {
	ID            string                 `json:"id"`
	Label         string                 `json:"label"`
	Kind          string                 `json:"kind"`
	Config        wireSourceConfig       `json:"config"`
	HasCredential bool                   `json:"hasCredential"`
	Health        wireSourceHealth       `json:"health"`
	Capabilities  wireSourceCapabilities `json:"capabilities"`
}

type wireSourceListResponse struct {
	Sources []wireSource `json:"sources"`
}

type wireSourceCredentialInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type wireSourceCreate struct {
	Label      string                     `json:"label"`
	Kind       string                     `json:"kind"`
	Config     wireSourceConfig           `json:"config"`
	Credential *wireSourceCredentialInput `json:"credential,omitempty"`
}

type wireSourceUpdate struct {
	Label      *string                    `json:"label,omitempty"`
	Config     *wireSourceConfig          `json:"config,omitempty"`
	Credential *wireSourceCredentialInput `json:"credential,omitempty"`
}

type wireFileReference struct {
	ReferenceID string `json:"referenceId"`
	Format      string `json:"format"`
	SizeBytes   *int64 `json:"sizeBytes"`
}

type wireSourceCandidate struct {
	Title         string            `json:"title"`
	Author        *string           `json:"author"`
	FileReference wireFileReference `json:"fileReference"`
	CoverURL      *string           `json:"coverUrl"`
}

type wireSourceCandidatePage struct {
	Items      []wireSourceCandidate `json:"items"`
	NextCursor *string               `json:"nextCursor"`
}

func mapSourceToWire(rec postgres.SourceRecord) wireSource {
	var cfg wireSourceConfig
	if rec.Kind == string(sources.KindLocalFolder) {
		bp := rec.ConfigBasePath
		cfg.BasePath = &bp
	} else if rec.Kind == string(sources.KindOPDS) {
		bu := rec.ConfigBaseURL
		cfg.BaseURL = &bu
	}

	var detail *string
	if rec.HealthDetail != "" {
		d := rec.HealthDetail
		detail = &d
	}

	return wireSource{
		ID:            rec.ID,
		Label:         rec.Label,
		Kind:          rec.Kind,
		Config:        cfg,
		HasCredential: rec.HasCredential(),
		Health: wireSourceHealth{
			Status:    rec.HealthStatus,
			CheckedAt: rec.HealthCheckedAt,
			Detail:    detail,
		},
		Capabilities: wireSourceCapabilities{
			CanList:     rec.Capabilities.CanList,
			CanSearch:   rec.Capabilities.CanSearch,
			CanDownload: rec.Capabilities.CanDownload,
		},
	}
}

func mapCandidateToWire(c sources.SourceCandidate) wireSourceCandidate {
	var sizeBytes *int64
	if c.FileReference.SizeBytes != nil {
		sz := *c.FileReference.SizeBytes
		sizeBytes = &sz
	}
	return wireSourceCandidate{
		Title:  c.Title,
		Author: c.Author,
		FileReference: wireFileReference{
			ReferenceID: c.FileReference.ReferenceID,
			Format:      c.FileReference.Format,
			SizeBytes:   sizeBytes,
		},
		CoverURL: c.CoverURL,
	}
}

func mapCandidatePageToWire(p sources.CandidatePage) wireSourceCandidatePage {
	items := make([]wireSourceCandidate, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, mapCandidateToWire(it))
	}
	return wireSourceCandidatePage{
		Items:      items,
		NextCursor: p.NextCursor,
	}
}

func extractSourceID(r *http.Request) string {
	id := r.PathValue("id")
	if id != "" {
		return id
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/sources/") {
		trimmed := strings.TrimPrefix(path, "/api/v1/sources/")
		if idx := strings.Index(trimmed, "/"); idx != -1 {
			return trimmed[:idx]
		}
		return trimmed
	}
	return ""
}

func hasControlCharInString(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func parseLimit(limitStr string) (int, error) {
	if limitStr == "" {
		return sources.DefaultLimit, nil
	}
	l, err := strconv.Atoi(limitStr)
	if err != nil || l < 1 || l > sources.MaxLimit {
		return 0, &domain.Error{Category: domain.InvalidInput, Message: "limit: must be an integer between 1 and 50"}
	}
	return l, nil
}

// CreateSourceHandler returns the HTTP handler for POST /api/v1/sources (backend-source-adapter.md FR-1).
func CreateSourceHandler(repo SourceRecordRepository, poolRef *PoolRef, sem *sources.Semaphore, idGen domain.IDGenerator, logger *slog.Logger) http.Handler {
	if idGen == nil {
		idGen = idgen.New()
	}
	if sem == nil {
		sem = sources.NewSemaphore(sources.DefaultOutboundLimit)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		var req wireSourceCreate
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", id)
			return
		}

		if err := domain.ValidateBoundedText("label", req.Label, 100); err != nil {
			WriteError(w, domain.InvalidInput, err.Error(), id)
			return
		}

		if !sources.ValidKind(req.Kind) {
			WriteError(w, domain.InvalidInput, "kind: must be one of local-folder, opds", id)
			return
		}

		var basePath, baseURL string
		var cred sources.Credential
		hasCred := req.Credential != nil

		if req.Kind == string(sources.KindLocalFolder) {
			if req.Config.BasePath == nil {
				WriteError(w, domain.InvalidInput, "config.basePath must not be empty", id)
				return
			}
			basePath = *req.Config.BasePath
			if err := sources.ValidateLocalFolderPath(basePath); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			if req.Credential != nil {
				WriteError(w, domain.InvalidInput, "credential: not supported for local-folder sources", id)
				return
			}
		} else if req.Kind == string(sources.KindOPDS) {
			if req.Config.BaseURL == nil {
				WriteError(w, domain.InvalidInput, "config.baseUrl must not be empty", id)
				return
			}
			baseURL = *req.Config.BaseURL
			if err := sources.ValidateOPDSBaseURL(baseURL); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			if req.Credential != nil {
				c, err := sources.NewCredential(req.Credential.Username, req.Credential.Password)
				if err != nil {
					WriteError(w, domain.InvalidInput, err.Error(), id)
					return
				}
				cred = c
			}
		}

		sourceID := idGen.NewID()
		sc, _ := poolRef.GetSourceCrypto()

		var prov sources.Provider
		if req.Kind == string(sources.KindLocalFolder) {
			p, err := local.New(sourceID, basePath, sc.Codec, logger)
			if err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			prov = p
		} else {
			prov = opds.New(opds.Config{
				SourceID:      sourceID,
				BaseURL:       baseURL,
				Credential:    cred,
				HasCredential: hasCred,
				Semaphore:     sem,
				Codec:         sc.Codec,
				Logger:        logger,
			})
		}

		probeResult := prov.Probe(r.Context())

		var ct, nonce []byte
		if hasCred {
			if sc.Encryptor == nil {
				WriteError(w, domain.Unavailable, "crypto service not ready", id)
				return
			}
			u, p := cred.Reveal()
			credJSON, _ := json.Marshal(wireSourceCredentialInput{
				Username: u,
				Password: p,
			})
			var encErr error
			ct, nonce, encErr = sc.Encryptor.Encrypt(credJSON)
			if encErr != nil {
				WriteError(w, domain.Internal, "failed to encrypt credential", id)
				return
			}
		}

		now := time.Now().UTC()
		rec := postgres.SourceRecord{
			ID:                   sourceID,
			Label:                req.Label,
			Kind:                 req.Kind,
			ConfigBasePath:       basePath,
			ConfigBaseURL:        baseURL,
			CredentialCiphertext: ct,
			CredentialNonce:      nonce,
			HealthStatus:         string(probeResult.Status),
			HealthDetail:         probeResult.Detail,
			HealthCheckedAt:      &now,
			SearchLinkURL:        probeResult.SearchLinkURL,
			Capabilities:         probeResult.Capabilities,
			CreatedAt:            now,
		}

		if err := repo.Create(r.Context(), rec); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		if logger != nil {
			logger.Info("health probe completed", "sourceId", rec.ID, "status", rec.HealthStatus, "detail", rec.HealthDetail)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(mapSourceToWire(rec))
	})
}

// ListSourcesHandler returns the HTTP handler for GET /api/v1/sources (backend-source-adapter.md FR-2).
func ListSourcesHandler(repo SourceRecordRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		recs, err := repo.List(r.Context())
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		wireSources := make([]wireSource, 0, len(recs))
		for _, rec := range recs {
			wireSources = append(wireSources, mapSourceToWire(rec))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(wireSourceListResponse{Sources: wireSources})
	})
}

// GetSourceHandler returns the HTTP handler for GET /api/v1/sources/{id} (backend-source-adapter.md FR-2).
func GetSourceHandler(repo SourceRecordRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceID := extractSourceID(r)
		if err := domain.ValidateBoundedText("id", sourceID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid source ID", id)
			return
		}

		rec, err := repo.Get(r.Context(), sourceID)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapSourceToWire(rec))
	})
}

// UpdateSourceHandler returns the HTTP handler for PATCH /api/v1/sources/{id} (backend-source-adapter.md FR-2).
func UpdateSourceHandler(repo SourceRecordRepository, poolRef *PoolRef, sem *sources.Semaphore, logger *slog.Logger) http.Handler {
	if sem == nil {
		sem = sources.NewSemaphore(sources.DefaultOutboundLimit)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceID := extractSourceID(r)
		if err := domain.ValidateBoundedText("id", sourceID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid source ID", id)
			return
		}

		rec, err := repo.Get(r.Context(), sourceID)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		var req wireSourceUpdate
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", id)
			return
		}

		newLabel := rec.Label
		if req.Label != nil {
			if err := domain.ValidateBoundedText("label", *req.Label, 100); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			newLabel = *req.Label
		}

		newBasePath := rec.ConfigBasePath
		newBaseURL := rec.ConfigBaseURL

		if req.Config != nil {
			if rec.Kind == string(sources.KindLocalFolder) {
				if req.Config.BasePath == nil {
					WriteError(w, domain.InvalidInput, "config.basePath must not be empty", id)
					return
				}
				newBasePath = *req.Config.BasePath
				if err := sources.ValidateLocalFolderPath(newBasePath); err != nil {
					WriteError(w, domain.InvalidInput, err.Error(), id)
					return
				}
			} else if rec.Kind == string(sources.KindOPDS) {
				if req.Config.BaseURL == nil {
					WriteError(w, domain.InvalidInput, "config.baseUrl must not be empty", id)
					return
				}
				newBaseURL = *req.Config.BaseURL
				if err := sources.ValidateOPDSBaseURL(newBaseURL); err != nil {
					WriteError(w, domain.InvalidInput, err.Error(), id)
					return
				}
			}
		}

		if err := repo.UpdateConfig(r.Context(), sourceID, newLabel, newBasePath, newBaseURL); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		var cred sources.Credential
		hasCred := rec.HasCredential()

		if req.Credential != nil {
			if rec.Kind == string(sources.KindLocalFolder) {
				WriteError(w, domain.InvalidInput, "credential: not supported for local-folder sources", id)
				return
			}
			c, err := sources.NewCredential(req.Credential.Username, req.Credential.Password)
			if err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			cred = c
			hasCred = true

			sc, ok := poolRef.GetSourceCrypto()
			if !ok || sc.Encryptor == nil {
				WriteError(w, domain.Unavailable, "crypto service not ready", id)
				return
			}
			u, p := cred.Reveal()
			credJSON, _ := json.Marshal(wireSourceCredentialInput{
				Username: u,
				Password: p,
			})
			ct, nonce, encErr := sc.Encryptor.Encrypt(credJSON)
			if encErr != nil {
				WriteError(w, domain.Internal, "failed to encrypt credential", id)
				return
			}
			if err := repo.SetCredential(r.Context(), sourceID, ct, nonce); err != nil {
				WriteError(w, domain.CategoryOf(err), err.Error(), id)
				return
			}
			rec.CredentialCiphertext = ct
			rec.CredentialNonce = nonce
		} else if rec.Kind == string(sources.KindOPDS) && hasCred {
			sc, ok := poolRef.GetSourceCrypto()
			if ok && sc.Encryptor != nil {
				pt, decErr := sc.Encryptor.Decrypt(rec.CredentialCiphertext, rec.CredentialNonce)
				if decErr == nil {
					var credInput wireSourceCredentialInput
					if err := json.Unmarshal(pt, &credInput); err == nil {
						c, cErr := sources.NewCredential(credInput.Username, credInput.Password)
						if cErr == nil {
							cred = c
						}
					}
				}
			}
		}

		sc, _ := poolRef.GetSourceCrypto()
		var prov sources.Provider
		if rec.Kind == string(sources.KindLocalFolder) {
			p, err := local.New(sourceID, newBasePath, sc.Codec, logger)
			if err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), id)
				return
			}
			prov = p
		} else {
			prov = opds.New(opds.Config{
				SourceID:       sourceID,
				BaseURL:        newBaseURL,
				Credential:     cred,
				HasCredential:  hasCred,
				SearchTemplate: rec.SearchLinkURL,
				Semaphore:      sem,
				Codec:          sc.Codec,
				Logger:         logger,
			})
		}

		probeResult := prov.Probe(r.Context())
		now := time.Now().UTC()
		if err := repo.UpdateHealth(r.Context(), sourceID, string(probeResult.Status), probeResult.Detail, now, probeResult.Capabilities, probeResult.SearchLinkURL); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		if logger != nil {
			logger.Info("health probe completed", "sourceId", sourceID, "status", string(probeResult.Status), "detail", probeResult.Detail)
		}

		updatedRec, err := repo.Get(r.Context(), sourceID)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapSourceToWire(updatedRec))
	})
}

// DeleteSourceHandler returns the HTTP handler for DELETE /api/v1/sources/{id} (backend-source-adapter.md FR-2, domain-source.md FR-6).
func DeleteSourceHandler(repo SourceRecordRepository, poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceID := extractSourceID(r)
		if err := domain.ValidateBoundedText("id", sourceID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid source ID", id)
			return
		}

		if _, err := repo.Get(r.Context(), sourceID); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		svc, ok := poolRef.GetSourceRemovalService()
		if !ok || svc == nil {
			WriteError(w, domain.Unavailable, "database not ready", id)
			return
		}

		if _, _, err := svc.Remove(r.Context(), domain.SourceID(sourceID), time.Now().UTC()); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

// HealthCheckSourceHandler returns the HTTP handler for POST /api/v1/sources/{id}/health-check (backend-source-adapter.md FR-6).
func HealthCheckSourceHandler(repo SourceRecordRepository, poolRef *PoolRef, sem *sources.Semaphore, logger *slog.Logger) http.Handler {
	if sem == nil {
		sem = sources.NewSemaphore(sources.DefaultOutboundLimit)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceID := extractSourceID(r)
		if err := domain.ValidateBoundedText("id", sourceID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid source ID", id)
			return
		}

		rec, err := repo.Get(r.Context(), sourceID)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		sc, _ := poolRef.GetSourceCrypto()
		var probeResult sources.ProbeResult

		if rec.Kind == string(sources.KindLocalFolder) {
			p, err := local.New(sourceID, rec.ConfigBasePath, sc.Codec, logger)
			if err != nil {
				probeResult = sources.ProbeResult{
					Status: sources.HealthUnreachable,
					Detail: sources.DetailPathNotFound,
				}
			} else {
				probeResult = p.Probe(r.Context())
			}
		} else if rec.Kind == string(sources.KindOPDS) {
			var cred sources.Credential
			hasCred := rec.HasCredential()
			decryptFailed := false

			if hasCred {
				if sc.Encryptor == nil {
					decryptFailed = true
				} else {
					pt, decErr := sc.Encryptor.Decrypt(rec.CredentialCiphertext, rec.CredentialNonce)
					if decErr != nil {
						decryptFailed = true
					} else {
						var credInput wireSourceCredentialInput
						if err := json.Unmarshal(pt, &credInput); err != nil {
							decryptFailed = true
						} else {
							c, cErr := sources.NewCredential(credInput.Username, credInput.Password)
							if cErr != nil {
								decryptFailed = true
							} else {
								cred = c
							}
						}
					}
				}
			}

			if decryptFailed {
				probeResult = sources.ProbeResult{
					Status: sources.HealthUnreachable,
					Detail: sources.DetailAuthRejected,
				}
			} else {
				p := opds.New(opds.Config{
					SourceID:       sourceID,
					BaseURL:        rec.ConfigBaseURL,
					Credential:     cred,
					HasCredential:  hasCred,
					SearchTemplate: rec.SearchLinkURL,
					Semaphore:      sem,
					Codec:          sc.Codec,
					Logger:         logger,
				})
				probeResult = p.Probe(r.Context())
			}
		} else {
			WriteError(w, domain.InvalidInput, "unsupported source kind", id)
			return
		}

		now := time.Now().UTC()
		if err := repo.UpdateHealth(r.Context(), sourceID, string(probeResult.Status), probeResult.Detail, now, probeResult.Capabilities, probeResult.SearchLinkURL); err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		if logger != nil {
			logger.Info("health probe completed", "sourceId", sourceID, "status", string(probeResult.Status), "detail", probeResult.Detail)
		}

		var detailPtr *string
		if probeResult.Detail != "" {
			detailPtr = &probeResult.Detail
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(wireSourceHealth{
			Status:    string(probeResult.Status),
			CheckedAt: &now,
			Detail:    detailPtr,
		})
	})
}

// BrowseSourceHandler returns the HTTP handler for GET /api/v1/sources/{id}/browse (backend-source-adapter.md FR-7).
func BrowseSourceHandler(repo SourceRecordRepository, poolRef *PoolRef, sem *sources.Semaphore, logger *slog.Logger) http.Handler {
	if sem == nil {
		sem = sources.NewSemaphore(sources.DefaultOutboundLimit)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceID := extractSourceID(r)
		if err := domain.ValidateBoundedText("id", sourceID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid source ID", id)
			return
		}

		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			WriteError(w, domain.InvalidInput, err.Error(), id)
			return
		}
		cursor := r.URL.Query().Get("cursor")

		rec, err := repo.Get(r.Context(), sourceID)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		sc, ok := poolRef.GetSourceCrypto()
		if !ok || sc.Codec == nil {
			WriteError(w, domain.Unavailable, "source is unavailable right now", id)
			return
		}

		var prov sources.Provider
		if rec.Kind == string(sources.KindLocalFolder) {
			p, err := local.New(sourceID, rec.ConfigBasePath, sc.Codec, logger)
			if err != nil {
				WriteError(w, domain.Unavailable, "source is unavailable right now", id)
				return
			}
			prov = p
		} else if rec.Kind == string(sources.KindOPDS) {
			var cred sources.Credential
			hasCred := rec.HasCredential()
			if hasCred {
				if sc.Encryptor == nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				pt, decErr := sc.Encryptor.Decrypt(rec.CredentialCiphertext, rec.CredentialNonce)
				if decErr != nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				var credInput wireSourceCredentialInput
				if err := json.Unmarshal(pt, &credInput); err != nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				c, cErr := sources.NewCredential(credInput.Username, credInput.Password)
				if cErr != nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				cred = c
			}
			prov = opds.New(opds.Config{
				SourceID:       sourceID,
				BaseURL:        rec.ConfigBaseURL,
				Credential:     cred,
				HasCredential:  hasCred,
				SearchTemplate: rec.SearchLinkURL,
				Semaphore:      sem,
				Codec:          sc.Codec,
				Logger:         logger,
			})
		} else {
			WriteError(w, domain.InvalidInput, "unsupported source kind", id)
			return
		}

		page, err := prov.List(r.Context(), cursor, limit)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapCandidatePageToWire(page))
	})
}

// SearchSourceHandler returns the HTTP handler for GET /api/v1/sources/{id}/search (backend-source-adapter.md FR-8).
func SearchSourceHandler(repo SourceRecordRepository, poolRef *PoolRef, sem *sources.Semaphore, logger *slog.Logger) http.Handler {
	if sem == nil {
		sem = sources.NewSemaphore(sources.DefaultOutboundLimit)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		sourceID := extractSourceID(r)
		if err := domain.ValidateBoundedText("id", sourceID, 100); err != nil {
			WriteError(w, domain.InvalidInput, "id: not a valid source ID", id)
			return
		}

		q := r.URL.Query().Get("q")
		if strings.TrimSpace(q) == "" {
			WriteError(w, domain.InvalidInput, "q: search query must be non-empty", id)
			return
		}
		if len([]rune(q)) > sources.MaxQueryLen {
			WriteError(w, domain.InvalidInput, "q: search query is too long", id)
			return
		}
		if hasControlCharInString(q) {
			WriteError(w, domain.InvalidInput, "q: search query contains control characters", id)
			return
		}

		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			WriteError(w, domain.InvalidInput, err.Error(), id)
			return
		}
		cursor := r.URL.Query().Get("cursor")

		rec, err := repo.Get(r.Context(), sourceID)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		if !rec.Capabilities.CanSearch {
			WriteError(w, domain.Conflict, "this source does not support search", id)
			return
		}

		sc, ok := poolRef.GetSourceCrypto()
		if !ok || sc.Codec == nil {
			WriteError(w, domain.Unavailable, "source is unavailable right now", id)
			return
		}

		var prov sources.Provider
		if rec.Kind == string(sources.KindLocalFolder) {
			p, err := local.New(sourceID, rec.ConfigBasePath, sc.Codec, logger)
			if err != nil {
				WriteError(w, domain.Unavailable, "source is unavailable right now", id)
				return
			}
			prov = p
		} else if rec.Kind == string(sources.KindOPDS) {
			var cred sources.Credential
			hasCred := rec.HasCredential()
			if hasCred {
				if sc.Encryptor == nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				pt, decErr := sc.Encryptor.Decrypt(rec.CredentialCiphertext, rec.CredentialNonce)
				if decErr != nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				var credInput wireSourceCredentialInput
				if err := json.Unmarshal(pt, &credInput); err != nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				c, cErr := sources.NewCredential(credInput.Username, credInput.Password)
				if cErr != nil {
					WriteError(w, domain.Unavailable, "source is unavailable right now", id)
					return
				}
				cred = c
			}
			prov = opds.New(opds.Config{
				SourceID:       sourceID,
				BaseURL:        rec.ConfigBaseURL,
				Credential:     cred,
				HasCredential:  hasCred,
				SearchTemplate: rec.SearchLinkURL,
				Semaphore:      sem,
				Codec:          sc.Codec,
				Logger:         logger,
			})
		} else {
			WriteError(w, domain.InvalidInput, "unsupported source kind", id)
			return
		}

		page, err := prov.Search(r.Context(), q, cursor, limit)
		if err != nil {
			WriteError(w, domain.CategoryOf(err), err.Error(), id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(mapCandidatePageToWire(page))
	})
}
