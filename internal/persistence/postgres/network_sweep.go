package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NetworkSweep executes the background lifecycle sweep for phase-13
// network pairing resources (backend-network-api.md FR-8, decision D-G).
// It must NOT run on any request path.
type NetworkSweep struct {
	pool        *pgxpool.Pool
	maxGrantTTL time.Duration
}

// NewNetworkSweep constructs a sweep runner.
func NewNetworkSweep(pool *pgxpool.Pool) *NetworkSweep {
	return &NetworkSweep{
		pool:        pool,
		maxGrantTTL: 10 * time.Minute,
	}
}

// SweepResults records counts of rows affected during one sweep run.
type SweepResults struct {
	ExpiredSessions  int64
	DeletedSessions  int64
	DeletedDevices   int64
	DeletedGrantJTIs int64
}

const (
	sweepExpireStaleSessionsSQL = `UPDATE pairing_sessions
		SET state = 'expired'
		WHERE state IN ('pending', 'verified') AND expires_at <= $1`

	sweepDeleteTerminalSessionsSQL = `DELETE FROM pairing_sessions
		WHERE state IN ('expired', 'consumed') AND expires_at <= $1`

	sweepDeleteAbandonedDevicesSQL = `DELETE FROM paired_devices
		WHERE owner_id IS NULL AND created_at <= $1`

	sweepDeleteSpentGrantJTIsSQL = `DELETE FROM enrolment_grant_jtis
		WHERE spent_at <= $1`
)

// SweepOnce runs one cycle of the four cleanup queries for now.
func (s *NetworkSweep) SweepOnce(ctx context.Context, now time.Time) (SweepResults, error) {
	exec := executorFrom(ctx, s.pool)
	var res SweepResults

	// 1. Mark pending/verified sessions past expires_at as expired
	tag, err := exec.Exec(ctx, sweepExpireStaleSessionsSQL, now)
	if err != nil {
		return res, TranslateError(err)
	}
	res.ExpiredSessions = tag.RowsAffected()

	// 2. Delete terminal sessions older than 24h (bounds initiator_ip retention)
	cutoff24h := now.Add(-24 * time.Hour)
	tag, err = exec.Exec(ctx, sweepDeleteTerminalSessionsSQL, cutoff24h)
	if err != nil {
		return res, TranslateError(err)
	}
	res.DeletedSessions = tag.RowsAffected()

	// 3. Delete provisional paired_devices (owner_id IS NULL) older than 1h
	cutoff1h := now.Add(-1 * time.Hour)
	tag, err = exec.Exec(ctx, sweepDeleteAbandonedDevicesSQL, cutoff1h)
	if err != nil {
		return res, TranslateError(err)
	}
	res.DeletedDevices = tag.RowsAffected()

	// 4. Delete enrolment_grant_jtis older than max grant TTL with safety margin
	cutoffGrant := now.Add(-s.maxGrantTTL - 1*time.Hour)
	tag, err = exec.Exec(ctx, sweepDeleteSpentGrantJTIsSQL, cutoffGrant)
	if err != nil {
		return res, TranslateError(err)
	}
	res.DeletedGrantJTIs = tag.RowsAffected()

	return res, nil
}

// Start launches a 5-minute ticker loop bound to ctx.
func (s *NetworkSweep) Start(ctx context.Context, interval time.Duration, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				res, err := s.SweepOnce(sweepCtx, t)
				cancel()
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
						return
					}
					if logger != nil {
						logger.Warn("network sweep failed", "error", err.Error())
					}
				} else if logger != nil && (res.ExpiredSessions > 0 || res.DeletedSessions > 0 || res.DeletedDevices > 0 || res.DeletedGrantJTIs > 0) {
					logger.Info("network sweep completed",
						"expired_sessions", res.ExpiredSessions,
						"deleted_sessions", res.DeletedSessions,
						"deleted_devices", res.DeletedDevices,
						"deleted_grant_jtis", res.DeletedGrantJTIs,
					)
				}
			}
		}
	}()
}
