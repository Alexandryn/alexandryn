//go:build darwin

package main

import (
	"context"
	"errors"

	"github.com/Alexandryn/alexandryn/internal/config"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// newObtainPostgres on macOS returns a startup function that selects between
// connecting to an existing DATABASE_URL or returning an error if local
// managed supervisor spawning is requested on macOS.
func newObtainPostgres() func(ctx context.Context, cfg *config.Config) error {
	return func(ctx context.Context, cfg *config.Config) error {
		return postgres.SelectStartupPath(ctx, cfg.DatabaseURL.Reveal(), spawnPostgresNotImplemented, connectPostgres(cfg))
	}
}

func spawnPostgresNotImplemented(context.Context) error {
	return errors.New("spawning a managed PostgreSQL instance on macOS is not implemented yet")
}
