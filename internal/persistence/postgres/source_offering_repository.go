package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// SourceOfferingRepository is internal/persistence/postgres's
// domain.SourceOfferingRepository implementation (T24, R7). Unlike
// Work/Author/Edition/Source, Save upserts on the real UNIQUE constraint
// on (source_id, edition_id, file_reference_format) — domain-source.md
// FR-2/FR-3's own identity for a SourceOffering — not on id: re-observing
// the same (Source, Edition, Format) tuple must update that row's
// FileReference/ObservedAt, matching SourceOffering.UniquenessKey(),
// rather than inserting a second row keyed on a fresh id. The row's own
// id is left unchanged on such an update, so a caller holding an earlier
// SourceOfferingID for that tuple can still FindByID it.
type SourceOfferingRepository struct {
	pool *pgxpool.Pool
}

func NewSourceOfferingRepository(pool *pgxpool.Pool) *SourceOfferingRepository {
	return &SourceOfferingRepository{pool: pool}
}

var _ domain.SourceOfferingRepository = (*SourceOfferingRepository)(nil)

func (r *SourceOfferingRepository) FindByID(ctx context.Context, id domain.SourceOfferingID) (*domain.SourceOffering, error) {
	exec := executorFrom(ctx, r.pool)

	row := exec.QueryRow(ctx,
		`SELECT source_id, edition_id, file_reference_id, file_reference_format, file_reference_size_bytes, observed_at
			FROM source_offerings WHERE id = $1`, string(id))
	return scanSourceOffering(row, id)
}

func (r *SourceOfferingRepository) FindBySource(ctx context.Context, sourceID domain.SourceID) ([]*domain.SourceOffering, error) {
	exec := executorFrom(ctx, r.pool)

	rows, err := exec.Query(ctx,
		`SELECT id, edition_id, file_reference_id, file_reference_format, file_reference_size_bytes, observed_at
			FROM source_offerings WHERE source_id = $1`, string(sourceID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.SourceOffering
	for rows.Next() {
		var id, editionID, referenceID, format string
		var sizeBytes *int64
		var observedAt time.Time
		if err := rows.Scan(&id, &editionID, &referenceID, &format, &sizeBytes, &observedAt); err != nil {
			return nil, TranslateError(err)
		}
		ref, err := domain.NewFileReference(referenceID, format, sizeBytes)
		if err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, domain.NewSourceOffering(domain.SourceOfferingID(id), sourceID, domain.EditionID(editionID), ref, observedAt))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

// FindByEdition returns every offering for editionID, most recently
// observed first (backend-reader-content.md FR-2's fallback order).
func (r *SourceOfferingRepository) FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.SourceOffering, error) {
	exec := executorFrom(ctx, r.pool)

	rows, err := exec.Query(ctx,
		`SELECT id, source_id, file_reference_id, file_reference_format, file_reference_size_bytes, observed_at
			FROM source_offerings WHERE edition_id = $1 ORDER BY observed_at DESC, id`, string(editionID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.SourceOffering
	for rows.Next() {
		var id, sourceID, referenceID, format string
		var sizeBytes *int64
		var observedAt time.Time
		if err := rows.Scan(&id, &sourceID, &referenceID, &format, &sizeBytes, &observedAt); err != nil {
			return nil, TranslateError(err)
		}
		ref, err := domain.NewFileReference(referenceID, format, sizeBytes)
		if err != nil {
			return nil, TranslateError(err)
		}
		result = append(result, domain.NewSourceOffering(domain.SourceOfferingID(id), domain.SourceID(sourceID), editionID, ref, observedAt))
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *SourceOfferingRepository) Save(ctx context.Context, o *domain.SourceOffering) error {
	exec := executorFrom(ctx, r.pool)

	ref := o.FileReference()
	_, err := exec.Exec(ctx, `INSERT INTO source_offerings
			(id, source_id, edition_id, file_reference_id, file_reference_format, file_reference_size_bytes, observed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (source_id, edition_id, file_reference_format) DO UPDATE SET
			file_reference_id = EXCLUDED.file_reference_id,
			file_reference_size_bytes = EXCLUDED.file_reference_size_bytes,
			observed_at = EXCLUDED.observed_at`,
		string(o.ID()), string(o.SourceID()), string(o.EditionID()), ref.ReferenceID, ref.Format, ref.SizeBytes, o.ObservedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *SourceOfferingRepository) Delete(ctx context.Context, id domain.SourceOfferingID) error {
	exec := executorFrom(ctx, r.pool)

	if _, err := exec.Exec(ctx, "DELETE FROM source_offerings WHERE id = $1", string(id)); err != nil {
		return TranslateError(err)
	}
	return nil
}

func scanSourceOffering(row pgx.Row, id domain.SourceOfferingID) (*domain.SourceOffering, error) {
	var sourceID, editionID, referenceID, format string
	var sizeBytes *int64
	var observedAt time.Time
	err := row.Scan(&sourceID, &editionID, &referenceID, &format, &sizeBytes, &observedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "source offering not found"}
		}
		return nil, TranslateError(err)
	}

	ref, err := domain.NewFileReference(referenceID, format, sizeBytes)
	if err != nil {
		return nil, TranslateError(err)
	}
	return domain.NewSourceOffering(id, domain.SourceID(sourceID), domain.EditionID(editionID), ref, observedAt), nil
}
