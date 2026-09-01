package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func containsDisallowedControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

// DiscoverSearchHandler handles GET /api/v1/discover (backend-metadata-adapter.md FR-1).
func DiscoverSearchHandler(client openlibrary.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		// Constitution §4: line-1 validation of query parameters.
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			WriteError(w, domain.InvalidInput, "q: search query must be non-empty", id)
			return
		}
		if len([]rune(q)) > 200 {
			WriteError(w, domain.InvalidInput, "q: search query exceeds maximum length of 200 characters", id)
			return
		}
		if containsDisallowedControlChars(q) {
			WriteError(w, domain.InvalidInput, "q: search query contains disallowed control characters", id)
			return
		}

		limit := 20
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			parsedLimit, err := strconv.Atoi(limitStr)
			if err != nil || parsedLimit < 1 || parsedLimit > 50 {
				WriteError(w, domain.InvalidInput, fmt.Sprintf("limit: invalid value %q — must be an integer between 1 and 50", limitStr), id)
				return
			}
			limit = parsedLimit
		}

		offset := 0
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			parsedOffset, err := strconv.Atoi(offsetStr)
			if err != nil || parsedOffset < 0 {
				WriteError(w, domain.InvalidInput, fmt.Sprintf("offset: invalid value %q — must be a non-negative integer", offsetStr), id)
				return
			}
			offset = parsedOffset
		}

		resp, err := client.Search(r.Context(), q, limit, offset)
		if err != nil {
			var domErr *domain.Error
			if errors.As(err, &domErr) {
				WriteError(w, domErr.Category, domErr.Message, id)
				return
			}
			WriteError(w, domain.Unavailable, "failed to perform search", id)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}

// DiscoverWorkDetailHandler handles GET /api/v1/discover/works/{openLibraryId} (FR-3).
func DiscoverWorkDetailHandler(client openlibrary.Client, cacheRepo postgres.MetadataCacheRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		// Constitution §4: line-1 validation of path parameter.
		rawID := r.PathValue("openLibraryId")
		openLibraryID := openlibrary.CleanKey(rawID)
		if !openlibrary.IsValidWorkKey(openLibraryID) {
			WriteError(w, domain.InvalidInput, fmt.Sprintf("openLibraryId: %q is not a valid Open Library work ID (must match ^OL[0-9]+W$)", rawID), id)
			return
		}

		// 1. Read-through cache check (backend-metadata-caching.md FR-2, FR-3)
		if cacheRepo != nil {
			if cached, hit, err := cacheRepo.GetWork(r.Context(), openLibraryID); err == nil && hit && cached != nil {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Metadata-Cache", "hit")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(cached)
				return
			}
		}

		// 2. Fetch upstream on cache miss
		detail, err := client.GetWork(r.Context(), openLibraryID)
		if err != nil {
			var domErr *domain.Error
			if errors.As(err, &domErr) {
				WriteError(w, domErr.Category, domErr.Message, id)
				return
			}
			WriteError(w, domain.Unavailable, "failed to fetch work details", id)
			return
		}

		// 3. Populate cache (resilient write)
		if cacheRepo != nil {
			_ = cacheRepo.SaveWork(r.Context(), openLibraryID, detail)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Metadata-Cache", "miss")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(detail)
	})
}

// DiscoverCoverHandler handles GET /api/v1/discover/covers/{coverId} (backend-metadata-caching.md FR-6).
func DiscoverCoverHandler(client openlibrary.Client, cacheRepo postgres.CoverCacheRepository) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := CorrelationIDFromContext(r.Context())

		// Constitution §4: line-1 validation of path parameter.
		coverIDStr := r.PathValue("coverId")
		coverID, err := strconv.ParseInt(coverIDStr, 10, 64)
		if err != nil || coverID <= 0 {
			WriteError(w, domain.InvalidInput, fmt.Sprintf("coverId: invalid value %q — must be a positive integer", coverIDStr), id)
			return
		}

		// 1. Check cover cache (FR-6)
		if cacheRepo != nil {
			filePath, contentType, isMissing, hit, err := cacheRepo.GetCover(r.Context(), coverID)
			if err == nil && hit {
				if isMissing {
					WriteError(w, domain.NotFound, "cover not found", id)
					return
				}
				if contentType == "" {
					contentType = "image/jpeg"
				}
				w.Header().Set("Content-Type", contentType)
				w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
				w.Header().Set("X-Metadata-Cache", "hit")
				http.ServeFile(w, r, filePath)
				return
			}
		}

		// 2. Fetch from Open Library Covers API
		data, contentType, err := client.FetchCover(r.Context(), coverID)
		if err != nil {
			var domErr *domain.Error
			if errors.As(err, &domErr) {
				if domErr.Category == domain.NotFound {
					if cacheRepo != nil {
						_ = cacheRepo.MarkMissing(r.Context(), coverID)
					}
				}
				WriteError(w, domErr.Category, domErr.Message, id)
				return
			}
			WriteError(w, domain.Unavailable, "failed to fetch cover image", id)
			return
		}

		if contentType == "" {
			contentType = "image/jpeg"
		}

		// 3. Save to local cache
		if cacheRepo != nil {
			savedPath, saveErr := cacheRepo.SaveCover(r.Context(), coverID, contentType, data)
			if saveErr == nil && savedPath != "" {
				w.Header().Set("Content-Type", contentType)
				w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
				w.Header().Set("X-Metadata-Cache", "miss")
				http.ServeFile(w, r, savedPath)
				return
			}
		}

		// Direct byte write fallback
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=2592000, immutable")
		w.Header().Set("X-Metadata-Cache", "miss")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
}
