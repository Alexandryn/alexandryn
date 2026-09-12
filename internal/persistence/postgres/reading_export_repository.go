package postgres

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// ReadingExportRepository is the bulk-read surface for reading data export —
// a straight read of the three reading tables, optionally scoped to one Work.
// It is read-only and performs no writes and no source calls.
type ReadingExportRepository struct {
	pool *pgxpool.Pool
}

func NewReadingExportRepository(pool *pgxpool.Pool) *ReadingExportRepository {
	return &ReadingExportRepository{pool: pool}
}

// ExportProgress is one Work's canonical ReadingProgress in export shape.
type ExportProgress struct {
	WorkID         string
	Percentage     float64
	Epoch          int64
	PrecisePosEdID *string
	PrecisePosCFI  *string
	ObservedAt     time.Time
}

// ExportMark is one bookmark or highlight in export shape (a highlight
// leaves StartCFI/EndCFI populated; a bookmark uses Position as StartCFI
// with EndCFI empty and Label set).
type ExportMark struct {
	ID        string
	EditionID string
	Kind      string // "bookmark" | "highlight"
	StartCFI  string
	EndCFI    string
	Label     string
	Note      string
	Category  string
	CreatedAt time.Time
}

// ListProgress returns the authenticated user's ReadingProgress in the
// active library, or just workID's when it is non-empty. It never returns
// another user's rows. Ordered by work id for a
// deterministic document.
func (r *ReadingExportRepository) ListProgress(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID string) ([]ExportProgress, error) {
	exec := executorFrom(ctx, r.pool)

	var clauses []string
	var args []any
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))
	clauses, args = appendOwnerScope(clauses, args, "library_id", string(libraryID))
	if workID != "" {
		args = append(args, workID)
		clauses = append(clauses, `work_id = $`+strconv.Itoa(len(args)))
	}
	q := `SELECT work_id, percentage, epoch, precise_position_edition_id, precise_position_value, observed_at
		FROM reading_progress
		WHERE ` + strings.Join(clauses, " AND ") + `
		ORDER BY work_id`

	rows, err := exec.Query(ctx, q, args...)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var out []ExportProgress
	for rows.Next() {
		var p ExportProgress
		if err := rows.Scan(&p.WorkID, &p.Percentage, &p.Epoch, &p.PrecisePosEdID, &p.PrecisePosCFI, &p.ObservedAt); err != nil {
			return nil, TranslateError(err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListMarks returns the authenticated user's bookmarks and highlights in
// the active library, or just those on the editions of workID when it is
// non-empty. It never returns another user's rows.
// Ordered by (edition_id, created_at, id).
func (r *ReadingExportRepository) ListMarks(ctx context.Context, userID domain.UserID, libraryID domain.LibraryID, workID string) ([]ExportMark, error) {
	exec := executorFrom(ctx, r.pool)

	var clauses []string
	var args []any
	clauses, args = appendOwnerScope(clauses, args, "user_id", string(userID))
	clauses, args = appendOwnerScope(clauses, args, "library_id", string(libraryID))
	scope := strings.Join(clauses, " AND ")
	workFilter := ``
	if workID != "" {
		args = append(args, workID)
		workFilter = ` AND edition_id IN (SELECT id FROM editions WHERE work_id = $` + strconv.Itoa(len(args)) + `)`
	}

	q := `SELECT id, edition_id, 'bookmark' AS kind, position AS start_cfi, '' AS end_cfi, label, '' AS note, '' AS category, created_at
		FROM bookmarks WHERE ` + scope + workFilter + `
		UNION ALL
		SELECT id, edition_id, 'highlight' AS kind, start_position AS start_cfi, end_position AS end_cfi, '' AS label, note, category, created_at
		FROM highlights WHERE ` + scope + workFilter + `
		ORDER BY edition_id, created_at, id`

	rows, err := exec.Query(ctx, q, args...)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var out []ExportMark
	for rows.Next() {
		var m ExportMark
		if err := rows.Scan(&m.ID, &m.EditionID, &m.Kind, &m.StartCFI, &m.EndCFI, &m.Label, &m.Note, &m.Category, &m.CreatedAt); err != nil {
			return nil, TranslateError(err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// WorkExists reports whether workID names an existing Work record.
func (r *ReadingExportRepository) WorkExists(ctx context.Context, workID string) (bool, error) {
	exec := executorFrom(ctx, r.pool)
	var exists bool
	err := exec.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM works WHERE id = $1)`, workID).Scan(&exists)
	if err != nil {
		return false, TranslateError(err)
	}
	return exists, nil
}
