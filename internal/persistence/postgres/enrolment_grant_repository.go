package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// EnrolmentGrantJTIRepository implements domain.EnrolmentGrantJTIRepository.
type EnrolmentGrantJTIRepository struct {
	pool *pgxpool.Pool
}

func NewEnrolmentGrantJTIRepository(pool *pgxpool.Pool) *EnrolmentGrantJTIRepository {
	return &EnrolmentGrantJTIRepository{pool: pool}
}

var _ domain.EnrolmentGrantJTIRepository = (*EnrolmentGrantJTIRepository)(nil)

// jti is the table's primary key, so the insert is the atomic claim:
// ON CONFLICT DO NOTHING means a concurrent second claim inserts no row.
const enrolmentGrantJTIClaimSQL = `INSERT INTO enrolment_grant_jtis (jti, spent_at) VALUES ($1, $2) ON CONFLICT (jti) DO NOTHING`

func (r *EnrolmentGrantJTIRepository) Claim(ctx context.Context, jti string, spentAt time.Time) (bool, error) {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, enrolmentGrantJTIClaimSQL, jti, spentAt)
	if err != nil {
		return false, TranslateError(err)
	}
	return tag.RowsAffected() == 1, nil
}
