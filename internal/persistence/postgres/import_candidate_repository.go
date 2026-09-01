package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// ImportCandidateStatus constants matching the 6-state lifecycle in backend-import-pipeline.md FR-3.
const (
	ImportCandidateStatusQueued       = "queued"
	ImportCandidateStatusPending      = "pending"
	ImportCandidateStatusAutoImported = "auto_imported"
	ImportCandidateStatusConfirmed    = "confirmed"
	ImportCandidateStatusRejected     = "rejected"
	ImportCandidateStatusFailed       = "failed"
)

// ValidImportCandidateStatus reports whether s is one of the six valid status values.
func ValidImportCandidateStatus(s string) bool {
	switch s {
	case ImportCandidateStatusQueued, ImportCandidateStatusPending,
		ImportCandidateStatusAutoImported, ImportCandidateStatusConfirmed,
		ImportCandidateStatusRejected, ImportCandidateStatusFailed:
		return true
	default:
		return false
	}
}

// ImportCandidateRecord is the persisted row for an import candidate (backend-import-pipeline.md FR-3).
// It is a persistence/staging type, not a domain aggregate.
type ImportCandidateRecord struct {
	ID                string
	SourceID          string
	FileReference     domain.FileReference
	Status            string
	ExtractedMetadata []byte // JSON bytes, nullable
	MatchCandidates   []byte // JSON bytes, nullable array
	JobID             *string
	LastError         *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// fileReferenceJSON is the wire/jsonb representation of a domain.FileReference.
type fileReferenceJSON struct {
	ID        string `json:"id"`
	Format    string `json:"format"`
	SizeBytes *int64 `json:"sizeBytes,omitempty"`
}

// ImportCandidateRepository persists ImportCandidateRecord rows.
type ImportCandidateRepository struct {
	pool *pgxpool.Pool
}

// NewImportCandidateRepository creates a new ImportCandidateRepository.
func NewImportCandidateRepository(pool *pgxpool.Pool) *ImportCandidateRepository {
	return &ImportCandidateRepository{pool: pool}
}

const importCandidateColumns = `id, source_id, file_reference, status,
	extracted_metadata, match_candidates, job_id, last_error, created_at, updated_at`

// Create inserts a new import candidate row.
func (r *ImportCandidateRepository) Create(ctx context.Context, rec ImportCandidateRecord) error {
	exec := executorFrom(ctx, r.pool)

	refJSON, err := json.Marshal(fileReferenceJSON{
		ID:        rec.FileReference.ReferenceID,
		Format:    rec.FileReference.Format,
		SizeBytes: rec.FileReference.SizeBytes,
	})
	if err != nil {
		return fmt.Errorf("marshalling file reference: %w", err)
	}

	_, err = exec.Exec(ctx, `INSERT INTO import_candidates (`+importCandidateColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		rec.ID, rec.SourceID, refJSON, rec.Status,
		nilIfEmpty(rec.ExtractedMetadata), nilIfEmpty(rec.MatchCandidates),
		rec.JobID, rec.LastError,
		orNow(rec.CreatedAt), orNow(rec.UpdatedAt))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// Get returns one import candidate by ID, or a NotFound *domain.Error.
func (r *ImportCandidateRepository) Get(ctx context.Context, id string) (ImportCandidateRecord, error) {
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, `SELECT `+importCandidateColumns+` FROM import_candidates WHERE id = $1`, id)
	rec, err := scanImportCandidate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ImportCandidateRecord{}, &domain.Error{Category: domain.NotFound, Message: "import candidate not found"}
		}
		return ImportCandidateRecord{}, TranslateError(err)
	}
	return rec, nil
}

// List returns candidates matching the optional sourceID and status filters, ordered by created_at, id.
func (r *ImportCandidateRepository) List(ctx context.Context, sourceID *string, status *string) ([]ImportCandidateRecord, error) {
	exec := executorFrom(ctx, r.pool)

	query := `SELECT ` + importCandidateColumns + ` FROM import_candidates WHERE 1=1`
	var args []any
	argIdx := 1

	if sourceID != nil && *sourceID != "" {
		query += fmt.Sprintf(` AND source_id = $%d`, argIdx)
		args = append(args, *sourceID)
		argIdx++
	}
	if status != nil && *status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *status)
		argIdx++
	}

	query += ` ORDER BY created_at, id`

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var out []ImportCandidateRecord
	for rows.Next() {
		rec, err := scanImportCandidate(rows)
		if err != nil {
			return nil, TranslateError(err)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return out, nil
}

// UpdateStatus updates the status and updated_at timestamp of a candidate.
func (r *ImportCandidateRepository) UpdateStatus(ctx context.Context, id string, status string, now time.Time) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, `UPDATE import_candidates SET status = $1, updated_at = $2 WHERE id = $3`,
		status, now, id)
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "import candidate not found"}
	}
	return nil
}

// UpdateExtractedAndMatches updates metadata, match candidates, status, and updated_at.
func (r *ImportCandidateRepository) UpdateExtractedAndMatches(ctx context.Context, id string, status string, extracted []byte, matches []byte, now time.Time) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, `UPDATE import_candidates
		SET status = $1, extracted_metadata = $2, match_candidates = $3, updated_at = $4
		WHERE id = $5`,
		status, nilIfEmpty(extracted), nilIfEmpty(matches), now, id)
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "import candidate not found"}
	}
	return nil
}

// UpdateFailed marks the candidate as failed with the given last error.
func (r *ImportCandidateRepository) UpdateFailed(ctx context.Context, id string, lastError string, now time.Time) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, `UPDATE import_candidates
		SET status = $1, last_error = $2, updated_at = $3
		WHERE id = $4`,
		ImportCandidateStatusFailed, lastError, now, id)
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "import candidate not found"}
	}
	return nil
}

// ExistsBySourceAndFileRefID checks whether a candidate row exists for (source_id, file_reference->>'id') in any status.
func (r *ImportCandidateRepository) ExistsBySourceAndFileRefID(ctx context.Context, sourceID string, fileRefID string) (bool, error) {
	exec := executorFrom(ctx, r.pool)
	var exists bool
	err := exec.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM import_candidates
		WHERE source_id = $1 AND (file_reference->>'id' = $2 OR file_reference->>'referenceId' = $2)
	)`, sourceID, fileRefID).Scan(&exists)
	if err != nil {
		return false, TranslateError(err)
	}
	return exists, nil
}

func scanImportCandidate(row pgx.Row) (ImportCandidateRecord, error) {
	var rec ImportCandidateRecord
	var refRaw []byte
	var extractedRaw, matchRaw []byte
	var jobID, lastError *string

	err := row.Scan(
		&rec.ID,
		&rec.SourceID,
		&refRaw,
		&rec.Status,
		&extractedRaw,
		&matchRaw,
		&jobID,
		&lastError,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if err != nil {
		return ImportCandidateRecord{}, err
	}

	var parsedRef fileReferenceJSON
	if err := json.Unmarshal(refRaw, &parsedRef); err != nil {
		return ImportCandidateRecord{}, fmt.Errorf("unmarshalling file reference: %w", err)
	}
	ref, err := domain.NewFileReference(parsedRef.ID, parsedRef.Format, parsedRef.SizeBytes)
	if err != nil {
		return ImportCandidateRecord{}, fmt.Errorf("building file reference: %w", err)
	}
	rec.FileReference = ref
	rec.ExtractedMetadata = extractedRaw
	rec.MatchCandidates = matchRaw
	rec.JobID = jobID
	rec.LastError = lastError

	return rec, nil
}

// ExistingEditionHit represents an owned edition in this library matching an ISBN (backend-import-pipeline.md FR-4(a)).
type ExistingEditionHit struct {
	EditionID string
	WorkID    string
	Title     string
	Author    string
	CoverURL  *string
	ISBN      string
}

// FindOwnedByISBN queries the database for owned editions matching isbn joined through library_entries (FR-4(a)).
func (r *ImportCandidateRepository) FindOwnedByISBN(ctx context.Context, isbn string) ([]ExistingEditionHit, error) {
	exec := executorFrom(ctx, r.pool)

	query := `SELECT e.id, e.work_id, w.title, COALESCE(string_agg(a.name, ', ' ORDER BY wa.ordering), ''), e.isbn
		FROM editions e
		JOIN works w ON e.work_id = w.id
		JOIN library_entries le ON le.edition_id = e.id
		LEFT JOIN work_authors wa ON wa.work_id = w.id
		LEFT JOIN authors a ON a.id = wa.author_id
		WHERE e.isbn = $1
		GROUP BY e.id, e.work_id, w.title, e.isbn
		ORDER BY e.id`

	rows, err := exec.Query(ctx, query, isbn)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var hits []ExistingEditionHit
	for rows.Next() {
		var hit ExistingEditionHit
		if err := rows.Scan(&hit.EditionID, &hit.WorkID, &hit.Title, &hit.Author, &hit.ISBN); err != nil {
			return nil, TranslateError(err)
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return hits, nil
}
