package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// ReadingPreferencesRepository is internal/persistence/postgres's
// domain.ReadingPreferencesRepository implementation. Settings
// is an opaque bag stored as JSONB rather than a fixed column set,
// marshalled/unmarshalled at this boundary so no JSON shape leaks
// into the domain type itself. Save upserts by device_id,
// ReadingPreferences' own natural primary key (it has no separate id
// field).
type ReadingPreferencesRepository struct {
	pool *pgxpool.Pool
}

func NewReadingPreferencesRepository(pool *pgxpool.Pool) *ReadingPreferencesRepository {
	return &ReadingPreferencesRepository{pool: pool}
}

var _ domain.ReadingPreferencesRepository = (*ReadingPreferencesRepository)(nil)

func (r *ReadingPreferencesRepository) FindByDevice(ctx context.Context, deviceID domain.DeviceID) (*domain.ReadingPreferences, error) {
	return r.FindByUserAndDevice(ctx, "", deviceID)
}

func (r *ReadingPreferencesRepository) FindByUserAndDevice(ctx context.Context, userID domain.UserID, deviceID domain.DeviceID) (*domain.ReadingPreferences, error) {
	exec := executorFrom(ctx, r.pool)

	var raw []byte
	query := `SELECT settings FROM reading_preferences WHERE device_id = $1 AND user_id = $2`
	err := exec.QueryRow(ctx, query, string(deviceID), string(userID)).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "reading preferences not found"}
		}
		return nil, TranslateError(err)
	}

	var settings map[string]string
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, TranslateError(err)
	}

	p := domain.NewReadingPreferences(deviceID)
	for k, v := range settings {
		p.Set(k, v)
	}
	return p, nil
}

func (r *ReadingPreferencesRepository) Save(ctx context.Context, p *domain.ReadingPreferences) error {
	return r.SaveForUser(ctx, "", p)
}

func (r *ReadingPreferencesRepository) SaveForUser(ctx context.Context, userID domain.UserID, p *domain.ReadingPreferences) error {
	exec := executorFrom(ctx, r.pool)

	raw, err := json.Marshal(p.Settings())
	if err != nil {
		return TranslateError(err)
	}

	_, err = exec.Exec(ctx, `INSERT INTO reading_preferences (user_id, device_id, settings)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, device_id) DO UPDATE SET settings = EXCLUDED.settings`,
		string(userID), string(p.DeviceID()), raw)
	if err != nil {
		return TranslateError(err)
	}
	return nil
}
