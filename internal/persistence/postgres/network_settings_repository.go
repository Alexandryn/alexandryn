package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// NetworkSettingsRepository implements domain.NetworkSettingsRepository
// (backend-network-api.md FR-7).
type NetworkSettingsRepository struct {
	pool *pgxpool.Pool
}

func NewNetworkSettingsRepository(pool *pgxpool.Pool) *NetworkSettingsRepository {
	return &NetworkSettingsRepository{pool: pool}
}

var _ domain.NetworkSettingsRepository = (*NetworkSettingsRepository)(nil)

const networkSettingsGetSQL = `SELECT host_name, remember_device_days, updated_at
	FROM network_settings WHERE id = 'default'`

func (r *NetworkSettingsRepository) Get(ctx context.Context) (*domain.NetworkSettings, error) {
	exec := executorFrom(ctx, r.pool)
	var hostName string
	var rememberDays int
	var updatedAt time.Time

	err := exec.QueryRow(ctx, networkSettingsGetSQL).Scan(&hostName, &rememberDays, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "network settings not found"}
		}
		return nil, TranslateError(err)
	}

	return &domain.NetworkSettings{
		HostName:           hostName,
		RememberDeviceDays: rememberDays,
		UpdatedAt:          updatedAt,
	}, nil
}

const networkSettingsUpsertSQL = `INSERT INTO network_settings (
		id, host_name, remember_device_days, updated_at
	) VALUES (
		'default', $1, $2, $3
	) ON CONFLICT (id) DO UPDATE SET
		host_name = EXCLUDED.host_name,
		remember_device_days = EXCLUDED.remember_device_days,
		updated_at = EXCLUDED.updated_at`

func (r *NetworkSettingsRepository) Upsert(ctx context.Context, s *domain.NetworkSettings) error {
	if s == nil {
		return &domain.Error{Category: domain.InvalidInput, Message: "network settings cannot be nil"}
	}
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, networkSettingsUpsertSQL,
		s.HostName,
		s.RememberDeviceDays,
		s.UpdatedAt,
	)
	if err != nil {
		return TranslateError(err)
	}
	return nil
}
