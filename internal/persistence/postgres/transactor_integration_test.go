//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// transactorTestPool opens a real *pgxpool.Pool against TEST_DATABASE_URL
// and gives the test a clean, dedicated fixture table — package postgres
// (white-box), not postgres_test, since executorFrom and querier are
// unexported implementation details this test needs direct access to,
// not part of the package's public API.
func transactorTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := NewPool(context.Background(), os.Getenv("TEST_DATABASE_URL"), 5)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS transactor_fixture"); err != nil {
		t.Fatalf("DROP TABLE: %v", err)
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE transactor_fixture (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
	return pool
}

func fixtureRowExists(t *testing.T, pool *pgxpool.Pool, id string) bool {
	t.Helper()
	var found string
	err := pool.QueryRow(context.Background(), "SELECT id FROM transactor_fixture WHERE id = $1", id).Scan(&found)
	if err == nil {
		return true
	}
	return false
}

// ADR 0021: a function that writes inside InTx and returns nil commits.
func TestTransactor_CommitsOnSuccess(t *testing.T) {
	ctx := context.Background()
	pool := transactorTestPool(t)
	tx := NewTransactor(pool)

	err := tx.InTx(ctx, func(ctx context.Context) error {
		exec := executorFrom(ctx, pool)
		_, err := exec.Exec(ctx, "INSERT INTO transactor_fixture (id) VALUES ($1)", "committed-row")
		return err
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if !fixtureRowExists(t, pool, "committed-row") {
		t.Fatal("row not visible after InTx returned nil — commit did not happen")
	}
}

// ADR 0021: a function that writes inside InTx and returns an error
// rolls back — the write must not be visible afterward.
func TestTransactor_RollsBackOnError(t *testing.T) {
	ctx := context.Background()
	pool := transactorTestPool(t)
	tx := NewTransactor(pool)
	sentinel := errors.New("deliberate failure after the write")

	err := tx.InTx(ctx, func(ctx context.Context) error {
		exec := executorFrom(ctx, pool)
		if _, err := exec.Exec(ctx, "INSERT INTO transactor_fixture (id) VALUES ($1)", "rolled-back-row"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want the sentinel to propagate unchanged", err)
	}
	if fixtureRowExists(t, pool, "rolled-back-row") {
		t.Fatal("row visible after InTx returned an error — rollback did not happen")
	}
}

// A nested call within the same InTx reuses the same transaction — two
// writes via executorFrom inside one InTx call either both commit or
// both roll back together, never independently.
func TestTransactor_MultipleWritesInOneInTxShareOneTransaction(t *testing.T) {
	ctx := context.Background()
	pool := transactorTestPool(t)
	tx := NewTransactor(pool)
	sentinel := errors.New("fail after both writes")

	err := tx.InTx(ctx, func(ctx context.Context) error {
		exec := executorFrom(ctx, pool)
		if _, err := exec.Exec(ctx, "INSERT INTO transactor_fixture (id) VALUES ($1)", "row-a"); err != nil {
			return err
		}
		if _, err := exec.Exec(ctx, "INSERT INTO transactor_fixture (id) VALUES ($1)", "row-b"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("InTx error = %v, want the sentinel", err)
	}
	if fixtureRowExists(t, pool, "row-a") || fixtureRowExists(t, pool, "row-b") {
		t.Fatal("one or both rows survived — both writes must roll back together, not independently")
	}
}

// executorFrom falls back to the pool when no transaction is in the
// context — a standalone call outside InTx still works, no signature
// change needed.
func TestExecutorFrom_FallsBackToPoolOutsideInTx(t *testing.T) {
	ctx := context.Background()
	pool := transactorTestPool(t)

	exec := executorFrom(ctx, pool)
	if _, err := exec.Exec(ctx, "INSERT INTO transactor_fixture (id) VALUES ($1)", "standalone-row"); err != nil {
		t.Fatalf("Exec via pool fallback: %v", err)
	}
	if !fixtureRowExists(t, pool, "standalone-row") {
		t.Fatal("standalone write via executorFrom's pool fallback did not persist")
	}
}
