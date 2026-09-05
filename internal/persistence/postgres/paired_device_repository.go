package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// PairedDeviceRepository implements domain.PairedDeviceRepository
// (backend-network-api.md FR-7 / FR-8 / FR-9).
type PairedDeviceRepository struct {
	pool *pgxpool.Pool
}

func NewPairedDeviceRepository(pool *pgxpool.Pool) *PairedDeviceRepository {
	return &PairedDeviceRepository{pool: pool}
}

var _ domain.PairedDeviceRepository = (*PairedDeviceRepository)(nil)

const pairedDeviceUpsertSQL = `INSERT INTO paired_devices (
		id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at, revoked_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8
	) ON CONFLICT (id) DO UPDATE SET
		owner_id = EXCLUDED.owner_id,
		label = EXCLUDED.label,
		device_class = EXCLUDED.device_class,
		enrolled_via = EXCLUDED.enrolled_via,
		last_seen_at = EXCLUDED.last_seen_at,
		revoked_at = EXCLUDED.revoked_at`

func (r *PairedDeviceRepository) Save(ctx context.Context, d *domain.PairedDevice) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, pairedDeviceUpsertSQL,
		string(d.ID()),
		string(d.Owner()),
		d.Label(),
		string(d.DeviceClass()),
		string(d.EnrolledVia()),
		d.CreatedAt(),
		d.LastSeenAt(),
		d.RevokedAt(),
	)
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

const pairedDeviceInsertProvisionalSQL = `INSERT INTO paired_devices (
		id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at, revoked_at, pairing_session_id
	) VALUES (
		$1, NULL, $2, $3, $4, $5, $5, NULL, $6
	)`

func (r *PairedDeviceRepository) InsertProvisional(
	ctx context.Context,
	id domain.DeviceID,
	label string,
	deviceClass domain.DeviceClass,
	enrolledVia domain.EnrolledVia,
	pairingSessionID domain.PairingSessionID,
	now time.Time,
) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, pairedDeviceInsertProvisionalSQL,
		string(id),
		label,
		string(deviceClass),
		string(enrolledVia),
		now,
		string(pairingSessionID),
	)
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

const pairedDeviceAssignOwnerBySessionSQL = `UPDATE paired_devices
	SET owner_id = $1
	WHERE pairing_session_id = $2 AND owner_id IS NULL`

func (r *PairedDeviceRepository) AssignOwnerByPairingSession(
	ctx context.Context,
	sessionID domain.PairingSessionID,
	owner domain.UserID,
) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, pairedDeviceAssignOwnerBySessionSQL, string(owner), string(sessionID))
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "provisional paired device not found for session"}
	}
	return nil
}

const pairedDeviceRevokeSQL = `UPDATE paired_devices
	SET revoked_at = $1
	WHERE id = $2 AND revoked_at IS NULL`

func (r *PairedDeviceRepository) Revoke(ctx context.Context, id domain.DeviceID, now time.Time) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, pairedDeviceRevokeSQL, now, string(id))
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "paired device not found or already revoked"}
	}
	return nil
}

const pairedDeviceSelectColumns = `id, owner_id, label, device_class, enrolled_via, created_at, last_seen_at, revoked_at`

const pairedDeviceFindByIDSQL = `SELECT ` + pairedDeviceSelectColumns + `
	FROM paired_devices WHERE id = $1`

func (r *PairedDeviceRepository) FindByID(ctx context.Context, id domain.DeviceID) (*domain.PairedDevice, error) {
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, pairedDeviceFindByIDSQL, string(id))
	return r.scanDevice(row)
}

const pairedDeviceFindBySessionIDSQL = `SELECT ` + pairedDeviceSelectColumns + `
	FROM paired_devices WHERE pairing_session_id = $1`

func (r *PairedDeviceRepository) FindByPairingSessionID(ctx context.Context, sessionID domain.PairingSessionID) (*domain.PairedDevice, error) {
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, pairedDeviceFindBySessionIDSQL, string(sessionID))
	return r.scanDevice(row)
}

const pairedDeviceFindByOwnerSQL = `SELECT ` + pairedDeviceSelectColumns + `
	FROM paired_devices WHERE owner_id = $1 ORDER BY created_at DESC`

func (r *PairedDeviceRepository) FindByOwner(ctx context.Context, owner domain.UserID) ([]*domain.PairedDevice, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, pairedDeviceFindByOwnerSQL, string(owner))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var devices []*domain.PairedDevice
	for rows.Next() {
		d, err := r.scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return devices, nil
}

func (r *PairedDeviceRepository) scanDevice(row pgx.Row) (*domain.PairedDevice, error) {
	var id, label, classStr, viaStr string
	var ownerID *string
	var createdAt, lastSeenAt time.Time
	var revokedAt *time.Time

	err := row.Scan(&id, &ownerID, &label, &classStr, &viaStr, &createdAt, &lastSeenAt, &revokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "paired device not found"}
		}
		return nil, TranslateError(err)
	}

	if ownerID == nil || *ownerID == "" {
		return nil, &domain.Error{Category: domain.NotFound, Message: "paired device is provisional (unassigned owner)"}
	}

	return domain.RehydratePairedDevice(
		domain.DeviceID(id),
		domain.UserID(*ownerID),
		label,
		domain.DeviceClass(classStr),
		domain.EnrolledVia(viaStr),
		createdAt,
		lastSeenAt,
		revokedAt,
	)
}
