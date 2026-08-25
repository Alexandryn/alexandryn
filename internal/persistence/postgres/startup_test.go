package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// FR-5: exactly one of spawn/connect runs, chosen by DATABASE_URL's
// presence — proven with call-recording fakes, no real process spawn or
// real Postgres.
func TestSelectStartupPath_EmptyDatabaseURLSpawns(t *testing.T) {
	var spawned, connected bool
	spawn := func(context.Context) error { spawned = true; return nil }
	connect := func(context.Context) error { connected = true; return nil }

	if err := postgres.SelectStartupPath(context.Background(), "", spawn, connect); err != nil {
		t.Fatalf("SelectStartupPath() error = %v, want nil", err)
	}
	if !spawned {
		t.Error("spawn was not called for an empty DATABASE_URL")
	}
	if connected {
		t.Error("connect was called for an empty DATABASE_URL, want spawn only")
	}
}

func TestSelectStartupPath_PresentDatabaseURLConnectsDirectly(t *testing.T) {
	var spawned, connected bool
	spawn := func(context.Context) error { spawned = true; return nil }
	connect := func(context.Context) error { connected = true; return nil }

	if err := postgres.SelectStartupPath(context.Background(), "postgres://fixture/proof", spawn, connect); err != nil {
		t.Fatalf("SelectStartupPath() error = %v, want nil", err)
	}
	if !connected {
		t.Error("connect was not called for a present DATABASE_URL")
	}
	if spawned {
		t.Error("spawn was called for a present DATABASE_URL, want connect only")
	}
}

func TestSelectStartupPath_PropagatesTheChosenPathsError(t *testing.T) {
	failing := errors.New("spawn failed")
	spawn := func(context.Context) error { return failing }
	connect := func(context.Context) error { return nil }

	if err := postgres.SelectStartupPath(context.Background(), "", spawn, connect); err != failing {
		t.Fatalf("SelectStartupPath() error = %v, want the spawn error propagated", err)
	}
}

// --- FR-8, portable subset (D4): the postgres argument list, a pure
// function, no macOS or real process needed ---

func TestPostgresArgs_IncludesTheDataDirectory(t *testing.T) {
	args := postgres.PostgresArgs("/home/user/.config/alexandryn/data", 5432)

	found := false
	for i, a := range args {
		if a == "-D" && i+1 < len(args) && args[i+1] == "/home/user/.config/alexandryn/data" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v don't include -D <dataDir>", args)
	}
}

// architecture-persistence.md FR-2: the bundled instance binds to a
// specific port (T25-D2's own OS-assigned port, passed in here), not
// Postgres's own compiled-in default.
func TestPostgresArgs_IncludesThePort(t *testing.T) {
	args := postgres.PostgresArgs("/home/user/.config/alexandryn/data", 54329)

	found := false
	for i, a := range args {
		if a == "-p" && i+1 < len(args) && args[i+1] == "54329" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args %v don't include -p 54329", args)
	}
}
