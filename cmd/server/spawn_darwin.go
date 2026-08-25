//go:build darwin

package main

import (
	"context"
	"errors"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// newObtainPostgres on macOS would spawn cmd/pg-supervisor
// (backend-persistence.md FR-8) instead of calling
// internal/persistence/postgres/supervisor directly — that mechanism
// isn't built yet (cmd/pg-supervisor is still an empty stub; macOS is
// explicitly out of scope for T25, D4,
// tasks/plan-t25-persistence-e2e.md). Failing loudly here, rather than
// silently no-op'ing, is backend-service-lifecycle.md FR-3's own
// requirement: a startup step that can't do its job fails, it doesn't
// pretend to succeed. connectPostgres (the DATABASE_URL-present branch)
// is unaffected — this only stubs the spawn branch.
func newObtainPostgres() func(ctx context.Context, cfg *config.Config) error {
	return func(ctx context.Context, cfg *config.Config) error {
		return postgres.SelectStartupPath(ctx, cfg.DatabaseURL.Reveal(), spawnPostgresNotImplemented, connectPostgres(cfg))
	}
}

func spawnPostgresNotImplemented(context.Context) error {
	return errors.New("spawning a managed PostgreSQL instance on macOS is not implemented yet (backend-persistence.md FR-8, macOS supervisor path)")
}
