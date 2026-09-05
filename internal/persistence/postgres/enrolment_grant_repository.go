package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// EnrolmentGrantJTIRepository implements domain.EnrolmentGrantJTIRepository
// (backend-network-api.md FR-7 / FR-9).
type EnrolmentGrantJTIRepository struct {
	pool *pgxpool.Pool
}

func NewEnrolmentGrantJTIRepository(pool *pgxpool.Pool) *EnrolmentGrantJTIRepository {
	return &EnrolmentGrantJTIRepository{pool: pool}
}

var _ domain.EnrolmentGrantJTIRepository = (*EnrolmentGrantJTIRepository)(nil)

const enrolmentGrantJTIRecordSQL = `INSERT INTO enrolment_grant_jtis (jti, spent_at) VALUES ($1, $2)`

func (r *EnrolmentGrantJTIRepository) Record(ctx context.Context, jti string, spentAt time.Time) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, enrolmentGrantJTIRecordSQL, jti, spentAt)
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

const enrolmentGrantJTIExistsSQL = `SELECT EXISTS (SELECT 1 FROM enrolment_grant_jtis WHERE jti = $1)`

func (r *EnrolmentGrantJTIRepository) Exists(ctx context.Context, jti string) (bool, error) {
	exec := executorFrom(ctx, r.pool)
	var exists bool
	err := exec.QueryRow(ctx, enrolmentGrantJTIExistsSQL, jti).Scan(&exists)
	if err != nil {
		return false, TranslateError(err)
	}
	return exists, nil
}
