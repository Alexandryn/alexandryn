package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// MFATicketJTIRepository implements domain.MFATicketJTIRepository — the
// single-use gate for MFA tickets (issue #189).
type MFATicketJTIRepository struct {
	pool *pgxpool.Pool
}

func NewMFATicketJTIRepository(pool *pgxpool.Pool) *MFATicketJTIRepository {
	return &MFATicketJTIRepository{pool: pool}
}

var _ domain.MFATicketJTIRepository = (*MFATicketJTIRepository)(nil)

// jti is the primary key, so ON CONFLICT DO NOTHING makes the insert the
// atomic claim: a concurrent second presentation inserts no row.
const mfaTicketJTIClaimSQL = `INSERT INTO mfa_ticket_jtis (jti, spent_at) VALUES ($1, $2) ON CONFLICT (jti) DO NOTHING`

func (r *MFATicketJTIRepository) Claim(ctx context.Context, jti string, spentAt time.Time) (bool, error) {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, mfaTicketJTIClaimSQL, jti, spentAt)
	if err != nil {
		return false, TranslateError(err)
	}
	return tag.RowsAffected() == 1, nil
}
