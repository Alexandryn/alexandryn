package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	readerapi "github.com/Alexandryn/alexandryn/internal/reader/api"
)

func parseFloat(s string) (float64, error) { return strconv.ParseFloat(s, 64) }

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// deviceIDHeader is the client-generated UUID v4 the progress and
// preferences endpoints require (backend-reading-api.md FR-1). Bookmark
// and highlight endpoints do not require it — those aggregates carry no
// DeviceID field.
const deviceIDHeader = "X-Device-Id"

func readingDeps(poolRef *PoolRef, w http.ResponseWriter, correlationID string) (ReadingAPI, bool) {
	deps, ok := poolRef.GetReadingAPI()
	if !ok {
		WriteError(w, domain.Unavailable, "the reading service is not ready yet", correlationID)
		return ReadingAPI{}, false
	}
	return deps, true
}

// readingScope resolves the authenticated user and validated active
// library for a reading request. Every reading handler scopes its
// repository calls to both (AUDIT-0012-C1, backend-reading-api.md FR-9):
// reading data — position, bookmarks, highlight notes — is private to its
// owner (constitution §8). The auth middleware populates the context; a
// missing user is a 401.
func readingScope(r *http.Request, w http.ResponseWriter, correlationID string) (domain.UserID, domain.LibraryID, bool) {
	user := UserFromContext(r.Context())
	if user == nil {
		WriteError(w, domain.Unauthorized, "authentication is required", correlationID)
		return "", "", false
	}
	return user.UserID, ActiveLibraryFromContext(r.Context()), true
}

// assertEditionInLibrary confirms editionID is owned in libID before a
// write is scoped to that library (AUDIT-0012-C1, review of PR #78): the
// read path (reader_content) and every write path must agree that reading
// data only references editions inside the caller's active library.
// "not in your library" and "does not exist" are the same 404 — no
// cross-library existence oracle.
func assertEditionInLibrary(deps ReadingAPI, r *http.Request, w http.ResponseWriter, editionID domain.EditionID, libID domain.LibraryID, correlationID string) bool {
	if deps.LibraryEntries == nil {
		WriteError(w, domain.Unavailable, "the reading service is not ready yet", correlationID)
		return false
	}
	ok, err := deps.LibraryEntries.EditionInLibrary(r.Context(), editionID, libID)
	if err != nil {
		writeDomainError(w, err, correlationID)
		return false
	}
	if !ok {
		WriteError(w, domain.NotFound, "no such edition in your library", correlationID)
		return false
	}
	return true
}

// assertWorkInLibrary is assertEditionInLibrary for a Work-keyed write
// (progress): the library must own at least one edition of the work.
func assertWorkInLibrary(deps ReadingAPI, r *http.Request, w http.ResponseWriter, workID domain.WorkID, libID domain.LibraryID, correlationID string) bool {
	if deps.LibraryEntries == nil {
		WriteError(w, domain.Unavailable, "the reading service is not ready yet", correlationID)
		return false
	}
	ok, err := deps.LibraryEntries.WorkInLibrary(r.Context(), workID, libID)
	if err != nil {
		writeDomainError(w, err, correlationID)
		return false
	}
	if !ok {
		WriteError(w, domain.NotFound, "no such book in your library", correlationID)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeJSONCapped marshals body and refuses to write a response over
// maxBytes (reading-data-export.md FR-6) — the 500 status keeps a
// browser from saving a truncated document as valid.
func writeJSONCapped(w http.ResponseWriter, status int, body any, maxBytes int, correlationID string) {
	raw, err := json.Marshal(body)
	if err != nil {
		WriteError(w, domain.Internal, "could not assemble the export document", correlationID)
		return
	}
	if len(raw) > maxBytes {
		WriteError(w, domain.Internal, "the export is too large; retry with ?workId= to scope it", correlationID)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

// --- progress -------------------------------------------------------------

type wirePrecisePosition struct {
	EditionID string `json:"editionId"`
	CFI       string `json:"cfi"`
}

type wireReadingProgress struct {
	Percentage      float64              `json:"percentage"`
	Epoch           int64                `json:"epoch"`
	PrecisePosition *wirePrecisePosition `json:"precisePosition"`
	ObservedAt      time.Time            `json:"observedAt"`
}

type wireProgressReport struct {
	Percentage      float64              `json:"percentage"`
	ObservedEpoch   int64                `json:"observedEpoch"`
	PrecisePosition *wirePrecisePosition `json:"precisePosition"`
	Override        bool                 `json:"override"`
}

func progressToWire(p *domain.ReadingProgress) *wireReadingProgress {
	if p == nil {
		return nil
	}
	out := &wireReadingProgress{
		Percentage: float64(p.Percentage()),
		Epoch:      p.Epoch(),
		ObservedAt: p.ObservedAt(),
	}
	if pos := p.PrecisePosition(); pos != nil {
		out.PrecisePosition = &wirePrecisePosition{EditionID: string(pos.EditionID), CFI: pos.Value}
	}
	return out
}

// ReadingProgressGetHandler: GET /api/v1/reading/works/{workId}/progress.
func ReadingProgressGetHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		workID := r.PathValue("workId")
		if workID == "" {
			WriteError(w, domain.InvalidInput, "workId is required", correlationID)
			return
		}
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}

		p, err := deps.Progress.FindByWorkAndUser(r.Context(), userID, libID, domain.WorkID(workID))
		if err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				writeJSON(w, http.StatusOK, map[string]any{"progress": nil})
				return
			}
			writeDomainError(w, err, correlationID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"progress": progressToWire(p)})
	})
}

// ReadingProgressReportHandler: POST /api/v1/reading/works/{workId}/progress.
func ReadingProgressReportHandler(poolRef *PoolRef, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}

		workID := r.PathValue("workId")
		if workID == "" {
			WriteError(w, domain.InvalidInput, "workId is required", correlationID)
			return
		}
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		if !assertWorkInLibrary(deps, r, w, domain.WorkID(workID), libID, correlationID) {
			return
		}

		deviceID := r.Header.Get(deviceIDHeader)
		if err := readerapi.ValidateDeviceID(deviceID); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		var req wireProgressReport
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", correlationID)
			return
		}
		if err := readerapi.ValidatePercentage(req.Percentage); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		if err := readerapi.ValidateObservedEpoch(req.ObservedEpoch); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		var pos *domain.PrecisePosition
		if req.PrecisePosition != nil {
			if err := readerapi.ValidateCFI(req.PrecisePosition.CFI); err != nil {
				writeDomainError(w, err, correlationID)
				return
			}
			if req.PrecisePosition.EditionID == "" {
				WriteError(w, domain.InvalidInput, "precisePosition.editionId is required", correlationID)
				return
			}
			pos = &domain.PrecisePosition{EditionID: domain.EditionID(req.PrecisePosition.EditionID), Value: req.PrecisePosition.CFI}
			if err := readerapi.CheckPrecisePositionWork(r.Context(), deps.Editions, domain.WorkID(workID), pos); err != nil {
				writeDomainError(w, err, correlationID)
				return
			}
		}

		pct, _ := domain.NewPercentage(req.Percentage)
		report := domain.ProgressReport{
			WorkID:          domain.WorkID(workID),
			Percentage:      pct,
			ObservedEpoch:   req.ObservedEpoch,
			PrecisePosition: pos,
			DeviceID:        domain.DeviceID(deviceID),
			ReportedAt:      now().UTC(),
		}

		result, outcome, err := readerapi.ReportProgress(r.Context(), deps.Transactor, progressStore{repo: deps.Progress, userID: userID, libraryID: libID}, idsAdapter{deps.IDs}, report, req.Override)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"progress": progressToWire(result),
			"outcome":  string(outcome),
		})
	})
}

// progressStore adapts a ReadingProgressRepository to readerapi's
// ProgressStore, closing over the authenticated user and active library so
// the row-lock read and the write are both scoped (AUDIT-0012-C1). The
// ProgressStore interface signature is unchanged — the scope is carried
// by the adapter, not threaded through readerapi.
type progressStore struct {
	repo      domain.ReadingProgressRepository
	userID    domain.UserID
	libraryID domain.LibraryID
}

func (s progressStore) FindByWorkForUpdate(ctx context.Context, workID domain.WorkID) (*domain.ReadingProgress, error) {
	return s.repo.FindByWorkAndUserForUpdate(ctx, s.userID, s.libraryID, workID)
}
func (s progressStore) Save(ctx context.Context, p *domain.ReadingProgress) error {
	return s.repo.SaveForUser(ctx, s.userID, s.libraryID, p)
}

type idsAdapter struct{ g domain.IDGenerator }

func (a idsAdapter) NewID() string { return a.g.NewID() }

// --- bookmarks -----------------------------------------------------------

type wireBookmark struct {
	ID        string    `json:"id"`
	EditionID string    `json:"editionId"`
	CFI       string    `json:"cfi"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"createdAt"`
}

func bookmarkToWire(b *domain.Bookmark) wireBookmark {
	return wireBookmark{
		ID: string(b.ID()), EditionID: string(b.EditionID()), CFI: b.Position(),
		Label: b.Label(), CreatedAt: b.CreatedAt(),
	}
}

// ReadingBookmarksListHandler: GET /api/v1/reading/editions/{editionId}/bookmarks.
func ReadingBookmarksListHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		editionID := r.PathValue("editionId")
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		list, err := deps.Bookmarks.FindByEditionAndUser(r.Context(), userID, libID, domain.EditionID(editionID))
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		out := make([]wireBookmark, 0, len(list))
		for _, b := range list {
			out = append(out, bookmarkToWire(b))
		}
		writeJSON(w, http.StatusOK, map[string]any{"bookmarks": out})
	})
}

// ReadingBookmarkCreateHandler: POST /api/v1/reading/editions/{editionId}/bookmarks.
func ReadingBookmarkCreateHandler(poolRef *PoolRef, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		editionID := r.PathValue("editionId")
		if editionID == "" {
			WriteError(w, domain.InvalidInput, "editionId is required", correlationID)
			return
		}
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		if !assertEditionInLibrary(deps, r, w, domain.EditionID(editionID), libID, correlationID) {
			return
		}

		var req struct {
			CFI   string  `json:"cfi"`
			Label *string `json:"label"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", correlationID)
			return
		}
		if err := readerapi.ValidateCFI(req.CFI); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		label := ""
		if req.Label != nil {
			label = *req.Label
		}
		if label != "" {
			if err := domain.ValidateBoundedText("label", label, 500); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), correlationID)
				return
			}
		}

		b := domain.NewBookmark(domain.BookmarkID(deps.IDs.NewID()), domain.EditionID(editionID), req.CFI, label, now().UTC())
		if err := deps.Bookmarks.SaveForUser(r.Context(), userID, libID, b); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"bookmark": bookmarkToWire(b)})
	})
}

// ReadingBookmarkDeleteHandler: DELETE /api/v1/reading/bookmarks/{bookmarkId}.
func ReadingBookmarkDeleteHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		id := domain.BookmarkID(r.PathValue("bookmarkId"))
		userID, _, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		if err := deps.Bookmarks.DeleteAndUser(r.Context(), userID, id); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// --- highlights ---------------------------------------------------------

type wireHighlight struct {
	ID        string    `json:"id"`
	EditionID string    `json:"editionId"`
	StartCFI  string    `json:"startCfi"`
	EndCFI    string    `json:"endCfi"`
	Note      string    `json:"note"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"createdAt"`
}

func highlightToWire(h *domain.Highlight) wireHighlight {
	return wireHighlight{
		ID: string(h.ID()), EditionID: string(h.EditionID()),
		StartCFI: h.StartPosition(), EndCFI: h.EndPosition(),
		Note: h.Note(), Category: h.Category(), CreatedAt: h.CreatedAt(),
	}
}

// ReadingHighlightsListHandler: GET /api/v1/reading/editions/{editionId}/highlights.
func ReadingHighlightsListHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		editionID := r.PathValue("editionId")
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		list, err := deps.Highlights.FindByEditionAndUser(r.Context(), userID, libID, domain.EditionID(editionID))
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		out := make([]wireHighlight, 0, len(list))
		for _, h := range list {
			out = append(out, highlightToWire(h))
		}
		writeJSON(w, http.StatusOK, map[string]any{"highlights": out})
	})
}

// ReadingHighlightCreateHandler: POST /api/v1/reading/editions/{editionId}/highlights.
func ReadingHighlightCreateHandler(poolRef *PoolRef, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		editionID := r.PathValue("editionId")
		if editionID == "" {
			WriteError(w, domain.InvalidInput, "editionId is required", correlationID)
			return
		}
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		if !assertEditionInLibrary(deps, r, w, domain.EditionID(editionID), libID, correlationID) {
			return
		}

		var req struct {
			StartCFI string  `json:"startCfi"`
			EndCFI   string  `json:"endCfi"`
			Note     *string `json:"note"`
			Category *string `json:"category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", correlationID)
			return
		}
		if err := readerapi.ValidateCFI(req.StartCFI); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		if err := readerapi.ValidateCFI(req.EndCFI); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		if readerapi.CFISortsBefore(req.EndCFI, req.StartCFI) {
			WriteError(w, domain.InvalidInput, "endCfi must not sort before startCfi", correlationID)
			return
		}
		note, category := "", ""
		if req.Note != nil {
			note = *req.Note
		}
		if req.Category != nil {
			category = *req.Category
		}
		if note != "" {
			if err := domain.ValidateBoundedText("note", note, 2000); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), correlationID)
				return
			}
		}
		if category != "" {
			if err := domain.ValidateBoundedText("category", category, 50); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), correlationID)
				return
			}
		}

		h := domain.NewHighlight(domain.HighlightID(deps.IDs.NewID()), domain.EditionID(editionID), req.StartCFI, req.EndCFI, note, category, now().UTC())
		if err := deps.Highlights.SaveForUser(r.Context(), userID, libID, h); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"highlight": highlightToWire(h)})
	})
}

// ReadingHighlightPatchHandler: PATCH /api/v1/reading/highlights/{highlightId}
// — updates note/category only, never the position (FR-7).
func ReadingHighlightPatchHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		id := domain.HighlightID(r.PathValue("highlightId"))
		userID, _, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		existing, err := deps.Highlights.FindByIDAndUser(r.Context(), userID, id)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		var req struct {
			Note     *string `json:"note"`
			Category *string `json:"category"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", correlationID)
			return
		}
		note, category := existing.Note(), existing.Category()
		if req.Note != nil {
			note = *req.Note
		}
		if req.Category != nil {
			category = *req.Category
		}
		if note != "" {
			if err := domain.ValidateBoundedText("note", note, 2000); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), correlationID)
				return
			}
		}
		if category != "" {
			if err := domain.ValidateBoundedText("category", category, 50); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), correlationID)
				return
			}
		}

		// Update note/category only — a PATCH must not move the highlight
		// into whatever library X-Library-Id currently names (PR #78
		// review). edition_id and library_id are left as stored.
		if err := deps.Highlights.UpdateNoteCategoryAndUser(r.Context(), userID, id, note, category); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		updated := domain.NewHighlight(existing.ID(), existing.EditionID(), existing.StartPosition(), existing.EndPosition(), note, category, existing.CreatedAt())
		writeJSON(w, http.StatusOK, map[string]any{"highlight": highlightToWire(updated)})
	})
}

// ReadingHighlightDeleteHandler: DELETE /api/v1/reading/highlights/{highlightId}.
func ReadingHighlightDeleteHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		id := domain.HighlightID(r.PathValue("highlightId"))
		userID, _, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		if err := deps.Highlights.DeleteAndUser(r.Context(), userID, id); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// --- preferences ------------------------------------------------------

type wirePreferences struct {
	Font        string  `json:"font"`
	FontSize    float64 `json:"fontSize"`
	LineSpacing float64 `json:"lineSpacing"`
	Theme       string  `json:"theme"`
	LayoutMode  string  `json:"layoutMode"`
	ColumnWidth string  `json:"columnWidth"`
}

// defaultPreferences is domain-reading.md FR-5's "new device starts from
// system defaults" made concrete, matching the atReader canvas.
func defaultPreferences() wirePreferences {
	return wirePreferences{
		Font: "serif", FontSize: 19, LineSpacing: 1.5,
		Theme: "light", LayoutMode: "paginated", ColumnWidth: "default",
	}
}

func preferencesToWire(p *domain.ReadingPreferences) wirePreferences {
	out := defaultPreferences()
	s := p.Settings()
	if v, ok := s["font"]; ok {
		out.Font = v
	}
	if v, ok := s["theme"]; ok {
		out.Theme = v
	}
	if v, ok := s["layoutMode"]; ok {
		out.LayoutMode = v
	}
	if v, ok := s["columnWidth"]; ok {
		out.ColumnWidth = v
	}
	if v, ok := s["fontSize"]; ok {
		if f, err := parseFloat(v); err == nil {
			out.FontSize = f
		}
	}
	if v, ok := s["lineSpacing"]; ok {
		if f, err := parseFloat(v); err == nil {
			out.LineSpacing = f
		}
	}
	return out
}

// ReadingPreferencesGetHandler: GET /api/v1/reading/preferences.
func ReadingPreferencesGetHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		deviceID := r.Header.Get(deviceIDHeader)
		if err := readerapi.ValidateDeviceID(deviceID); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		userID, _, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		p, err := deps.Preferences.FindByUserAndDevice(r.Context(), userID, domain.DeviceID(deviceID))
		if err != nil {
			if domain.CategoryOf(err) == domain.NotFound {
				writeJSON(w, http.StatusOK, map[string]any{"preferences": defaultPreferences()})
				return
			}
			writeDomainError(w, err, correlationID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"preferences": preferencesToWire(p)})
	})
}

// ReadingPreferencesPutHandler: PUT /api/v1/reading/preferences.
func ReadingPreferencesPutHandler(poolRef *PoolRef) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		deviceID := r.Header.Get(deviceIDHeader)
		if err := readerapi.ValidateDeviceID(deviceID); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		userID, _, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}
		var req wirePreferences
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, domain.InvalidInput, "request body must be a valid JSON object", correlationID)
			return
		}
		if err := validatePreferences(req); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		p := domain.NewReadingPreferences(domain.DeviceID(deviceID))
		p.Set("font", req.Font)
		p.Set("theme", req.Theme)
		p.Set("layoutMode", req.LayoutMode)
		p.Set("columnWidth", req.ColumnWidth)
		p.Set("fontSize", formatFloat(req.FontSize))
		p.Set("lineSpacing", formatFloat(req.LineSpacing))

		if err := deps.Preferences.SaveForUser(r.Context(), userID, p); err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"preferences": preferencesToWire(p)})
	})
}

func validatePreferences(p wirePreferences) error {
	if p.Font != "" {
		if err := domain.ValidateBoundedText("font", p.Font, 60); err != nil {
			return &domain.Error{Category: domain.InvalidInput, Message: err.Error()}
		}
	}
	switch p.Theme {
	case "light", "sepia", "dark":
	default:
		return &domain.Error{Category: domain.InvalidInput, Message: "theme must be light, sepia, or dark"}
	}
	switch p.LayoutMode {
	case "paginated", "scroll":
	default:
		return &domain.Error{Category: domain.InvalidInput, Message: "layoutMode must be paginated or scroll"}
	}
	switch p.ColumnWidth {
	case "narrow", "default", "wide":
	default:
		return &domain.Error{Category: domain.InvalidInput, Message: "columnWidth must be narrow, default, or wide"}
	}
	if p.FontSize < 12 || p.FontSize > 32 {
		return &domain.Error{Category: domain.InvalidInput, Message: "fontSize must be between 12 and 32"}
	}
	if p.LineSpacing < 1.0 || p.LineSpacing > 2.5 {
		return &domain.Error{Category: domain.InvalidInput, Message: "lineSpacing must be between 1.0 and 2.5"}
	}
	return nil
}
