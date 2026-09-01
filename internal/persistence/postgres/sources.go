package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// SourceRecord is the full persisted row for a phase-08 source: what
// domain-source.md deliberately keeps out of the domain model
// (backend-source-adapter.md — kind-specific config, an encrypted
// credential, health and capability state). It is a persistence-layer
// type, not a domain aggregate; the thin domain.SourceRepository still
// owns identity/label/capabilities for SourceRemovalService's atomic
// cascade.
type SourceRecord struct {
	ID                   string
	Label                string
	Kind                 string
	ConfigBasePath       string
	ConfigBaseURL        string
	CredentialCiphertext []byte // nil when the source has no credential
	CredentialNonce      []byte
	HealthStatus         string
	HealthDetail         string // "" means SQL NULL
	HealthCheckedAt      *time.Time
	SearchLinkURL        string // "" means SQL NULL
	Capabilities         domain.SourceCapabilities
	CreatedAt            time.Time
}

// HasCredential reports whether an encrypted credential is stored.
func (r SourceRecord) HasCredential() bool { return len(r.CredentialCiphertext) > 0 }

// SourceRecordRepository persists SourceRecord rows. It exposes no
// Delete — removing a source goes through domain.SourceRemovalService so
// the SourceOffering cascade is atomic (domain-source.md FR-6).
type SourceRecordRepository struct {
	pool *pgxpool.Pool
}

func NewSourceRecordRepository(pool *pgxpool.Pool) *SourceRecordRepository {
	return &SourceRecordRepository{pool: pool}
}

const sourceColumns = `id, label, kind, config_base_path, config_base_url,
	credential_ciphertext, credential_nonce,
	health_status, health_detail, health_checked_at, search_link_url,
	can_list, can_search, can_download, created_at`

// Create inserts a new source row.
func (r *SourceRecordRepository) Create(ctx context.Context, rec SourceRecord) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `INSERT INTO sources (`+sourceColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		rec.ID, rec.Label, rec.Kind, rec.ConfigBasePath, rec.ConfigBaseURL,
		nilIfEmpty(rec.CredentialCiphertext), nilIfEmpty(rec.CredentialNonce),
		statusOrUnknown(rec.HealthStatus), nullIfEmpty(rec.HealthDetail), rec.HealthCheckedAt, nullIfEmpty(rec.SearchLinkURL),
		rec.Capabilities.CanList, rec.Capabilities.CanSearch, rec.Capabilities.CanDownload,
		orNow(rec.CreatedAt))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// Get returns one source, or a NotFound *domain.Error.
func (r *SourceRecordRepository) Get(ctx context.Context, id string) (SourceRecord, error) {
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, `SELECT `+sourceColumns+` FROM sources WHERE id = $1`, id)
	rec, err := scanSource(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SourceRecord{}, &domain.Error{Category: domain.NotFound, Message: "source not found"}
		}
		return SourceRecord{}, TranslateError(err)
	}
	return rec, nil
}

// List returns every source, oldest first.
func (r *SourceRecordRepository) List(ctx context.Context) ([]SourceRecord, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, `SELECT `+sourceColumns+` FROM sources ORDER BY created_at, id`)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var out []SourceRecord
	for rows.Next() {
		rec, err := scanSource(rows)
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

// UpdateConfig updates a source's label and kind-specific config
// (backend-source-adapter.md FR-2). It does not touch the credential or
// health state.
func (r *SourceRecordRepository) UpdateConfig(ctx context.Context, id, label, basePath, baseURL string) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, `UPDATE sources
		SET label = $2, config_base_path = $3, config_base_url = $4
		WHERE id = $1`, id, label, basePath, baseURL)
	if err != nil {
		return TranslateError(err)
	}
	return notFoundIfNoRows(tag.RowsAffected())
}

// SetCredential replaces or clears a source's stored credential
// (FR-2 — a credential update replaces the value entirely). Pass nil
// ciphertext and nonce to clear it.
func (r *SourceRecordRepository) SetCredential(ctx context.Context, id string, ciphertext, nonce []byte) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, `UPDATE sources
		SET credential_ciphertext = $2, credential_nonce = $3
		WHERE id = $1`, id, nilIfEmpty(ciphertext), nilIfEmpty(nonce))
	if err != nil {
		return TranslateError(err)
	}
	return notFoundIfNoRows(tag.RowsAffected())
}

// UpdateHealth writes the result of a health check: reachability, the
// re-detected capabilities, and the origin-validated search link
// (FR-5, FR-6, FR-11).
func (r *SourceRecordRepository) UpdateHealth(ctx context.Context, id, status, detail string, checkedAt time.Time, caps domain.SourceCapabilities, searchLinkURL string) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, `UPDATE sources SET
		health_status = $2, health_detail = $3, health_checked_at = $4,
		search_link_url = $5, can_list = $6, can_search = $7, can_download = $8
		WHERE id = $1`,
		id, statusOrUnknown(status), nullIfEmpty(detail), checkedAt,
		nullIfEmpty(searchLinkURL), caps.CanList, caps.CanSearch, caps.CanDownload)
	if err != nil {
		return TranslateError(err)
	}
	return notFoundIfNoRows(tag.RowsAffected())
}

// CountWithCredential reports how many sources hold an encrypted
// credential — crypto.LoadOrCreateKey's first-run vs. lost-key check
// (FR-13).
func (r *SourceRecordRepository) CountWithCredential(ctx context.Context) (int, error) {
	if r == nil || r.pool == nil {
		return 0, nil
	}
	exec := executorFrom(ctx, r.pool)
	var n int
	err := exec.QueryRow(ctx, `SELECT count(*) FROM sources WHERE credential_ciphertext IS NOT NULL`).Scan(&n)
	if err != nil {
		return 0, TranslateError(err)
	}
	return n, nil
}

// scannable is satisfied by both pgx.Row and pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
}

func scanSource(row scannable) (SourceRecord, error) {
	var rec SourceRecord
	var detail, searchLink *string
	err := row.Scan(
		&rec.ID, &rec.Label, &rec.Kind, &rec.ConfigBasePath, &rec.ConfigBaseURL,
		&rec.CredentialCiphertext, &rec.CredentialNonce,
		&rec.HealthStatus, &detail, &rec.HealthCheckedAt, &searchLink,
		&rec.Capabilities.CanList, &rec.Capabilities.CanSearch, &rec.Capabilities.CanDownload,
		&rec.CreatedAt,
	)
	if err != nil {
		return SourceRecord{}, err
	}
	if detail != nil {
		rec.HealthDetail = *detail
	}
	if searchLink != nil {
		rec.SearchLinkURL = *searchLink
	}
	return rec, nil
}

func notFoundIfNoRows(affected int64) error {
	if affected == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "source not found"}
	}
	return nil
}

func nilIfEmpty(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func statusOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func orNow(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}
