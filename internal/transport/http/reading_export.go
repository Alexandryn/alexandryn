package http

import (
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// exportSchemaVersion is the versioned envelope's contract (reading-data-export.md
// FR-1) — a change to the document shape bumps this, never edits in place.
const exportSchemaVersion = 1

// exportMaxBytes bounds the assembled document (FR-6) — a reasoned
// placeholder; a realistic annotated library stays well under it.
const exportMaxBytes = 25 * 1024 * 1024

type exportDoc struct {
	SchemaVersion int             `json:"schemaVersion"`
	ExportedAt    time.Time       `json:"exportedAt"`
	Works         []exportWork    `json:"works"`
	Editions      []exportEdition `json:"editions"`
}

type exportWork struct {
	WorkID   string              `json:"workId"`
	Progress *exportProgressJSON `json:"progress"`
}

type exportProgressJSON struct {
	Percentage      float64              `json:"percentage"`
	Epoch           int64                `json:"epoch"`
	PrecisePosition *wirePrecisePosition `json:"precisePosition"`
	ObservedAt      time.Time            `json:"observedAt"`
}

type exportEdition struct {
	EditionID  string          `json:"editionId"`
	Bookmarks  []wireBookmark  `json:"bookmarks"`
	Highlights []wireHighlight `json:"highlights"`
}

// ReadingExportHandler: GET /api/v1/reading/export (+ ?workId=) —
// reading-data-export.md FR-1/FR-2. Read-only; no writes, no source
// calls; logs counts only, never a title, CFI, label, or note (§8).
func ReadingExportHandler(poolRef *PoolRef, logger *slog.Logger, now func() time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		correlationID := CorrelationIDFromContext(r.Context())
		deps, ok := readingDeps(poolRef, w, correlationID)
		if !ok {
			return
		}
		if deps.Export == nil {
			WriteError(w, domain.Unavailable, "export is not available", correlationID)
			return
		}
		userID, libID, ok := readingScope(r, w, correlationID)
		if !ok {
			return
		}

		workID := r.URL.Query().Get("workId")
		if workID != "" {
			if err := domain.ValidateBoundedText("workId", workID, 200); err != nil {
				WriteError(w, domain.InvalidInput, err.Error(), correlationID)
				return
			}
			exists, err := deps.Export.WorkExists(r.Context(), workID)
			if err != nil {
				writeDomainError(w, err, correlationID)
				return
			}
			if !exists {
				WriteError(w, domain.NotFound, "no work with that id", correlationID)
				return
			}
		}

		rawProgress, err := deps.Export.ListProgress(r.Context(), userID, libID, workID)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}
		rawMarks, err := deps.Export.ListMarks(r.Context(), userID, libID, workID)
		if err != nil {
			writeDomainError(w, err, correlationID)
			return
		}

		progress := make([]exportProgressRow, 0, len(rawProgress))
		for _, p := range rawProgress {
			progress = append(progress, exportProgressRow{
				WorkID: p.WorkID, Percentage: p.Percentage, Epoch: p.Epoch,
				PrecisePosEdID: p.PrecisePosEdID, PrecisePosCFI: p.PrecisePosCFI, ObservedAt: p.ObservedAt,
			})
		}
		marks := make([]exportMarkRow, 0, len(rawMarks))
		for _, m := range rawMarks {
			marks = append(marks, exportMarkRow{
				ID: m.ID, EditionID: m.EditionID, Kind: m.Kind, StartCFI: m.StartCFI, EndCFI: m.EndCFI,
				Label: m.Label, Note: m.Note, Category: m.Category, CreatedAt: m.CreatedAt,
			})
		}

		doc := buildExportDoc(now().UTC(), progress, marks)

		if logger != nil {
			logger.InfoContext(r.Context(), "reading data exported",
				"correlationId", correlationID,
				"scoped", workID != "",
				"works", len(doc.Works),
				"editions", len(doc.Editions),
			)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="alexandryn-reading-export.json"`)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		writeJSONCapped(w, http.StatusOK, doc, exportMaxBytes, correlationID)
	})
}

func buildExportDoc(exportedAt time.Time, progress []exportProgressRow, marks []exportMarkRow) exportDoc {
	doc := exportDoc{SchemaVersion: exportSchemaVersion, ExportedAt: exportedAt, Works: []exportWork{}, Editions: []exportEdition{}}

	for _, p := range progress {
		ew := exportWork{WorkID: p.WorkID}
		pj := &exportProgressJSON{Percentage: p.Percentage, Epoch: p.Epoch, ObservedAt: p.ObservedAt}
		if p.PrecisePosEdID != nil && p.PrecisePosCFI != nil {
			pj.PrecisePosition = &wirePrecisePosition{EditionID: *p.PrecisePosEdID, CFI: *p.PrecisePosCFI}
		}
		ew.Progress = pj
		doc.Works = append(doc.Works, ew)
	}

	byEdition := map[string]*exportEdition{}
	var order []string
	for _, m := range marks {
		e, ok := byEdition[m.EditionID]
		if !ok {
			e = &exportEdition{EditionID: m.EditionID, Bookmarks: []wireBookmark{}, Highlights: []wireHighlight{}}
			byEdition[m.EditionID] = e
			order = append(order, m.EditionID)
		}
		switch m.Kind {
		case "bookmark":
			e.Bookmarks = append(e.Bookmarks, wireBookmark{
				ID: m.ID, EditionID: m.EditionID, CFI: m.StartCFI, Label: m.Label, CreatedAt: m.CreatedAt,
			})
		case "highlight":
			e.Highlights = append(e.Highlights, wireHighlight{
				ID: m.ID, EditionID: m.EditionID, StartCFI: m.StartCFI, EndCFI: m.EndCFI,
				Note: m.Note, Category: m.Category, CreatedAt: m.CreatedAt,
			})
		}
	}
	sort.Strings(order)
	for _, id := range order {
		doc.Editions = append(doc.Editions, *byEdition[id])
	}
	return doc
}

// exportProgressRow / exportMarkRow decouple the handler from the
// postgres-specific export row types (constitution §3 — normalise at the
// boundary).
type exportProgressRow struct {
	WorkID         string
	Percentage     float64
	Epoch          int64
	PrecisePosEdID *string
	PrecisePosCFI  *string
	ObservedAt     time.Time
}

type exportMarkRow struct {
	ID        string
	EditionID string
	Kind      string
	StartCFI  string
	EndCFI    string
	Label     string
	Note      string
	Category  string
	CreatedAt time.Time
}
