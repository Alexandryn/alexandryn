package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// querier is the minimal shape both *pgxpool.Pool and pgx.Tx satisfy.
// Every repository method reads its executor through executorFrom rather
// than accepting a pool or a transaction directly, so the same method
// serves both transactional and standalone callers with no signature
// change.
//
//nolint:unused // consumed by transactor_integration_test.go and repository methods
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txContextKey struct{}

// executorFrom returns the transaction InTx placed in ctx, if any, or
// pool otherwise — the shared helper that ensures operations participate in
// an in-flight transaction when present. Every repository method in this
// package reads its executor through this function.
//
//nolint:unused
func executorFrom(ctx context.Context, pool *pgxpool.Pool) querier {
	if tx, ok := ctx.Value(txContextKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// Transactor is the PostgreSQL implementation of domain.Transactor — the
// mechanism a domain service holding more than one repository interface
// uses to compose a cross-aggregate operation into one atomic unit (such
// as SourceRemovalService's cascade).
type Transactor struct {
	pool *pgxpool.Pool
}

func NewTransactor(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool}
}

var _ domain.Transactor = (*Transactor)(nil)

// InTx begins a transaction, places it in the context passed to fn via
// executorFrom's own lookup key, and commits on fn's success or rolls
// back on its error. A panic inside fn is also rolled back, then
// re-panicked — never silently swallowed into a rolled-back-but-reported-
// successful state.
func (t *Transactor) InTx(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return TranslateError(err)
	}

	txCtx := context.WithValue(ctx, txContextKey{}, tx)

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return TranslateError(err)
	}
	return nil
}
