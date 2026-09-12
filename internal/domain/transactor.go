package domain

import "context"

// Transactor lets a domain service compose an operation spanning more
// than one repository into a single atomic unit, without any
// persistence-specific type ever appearing in an internal/domain
// signature. The implementation in
// internal/persistence/postgres begins a transaction, places it in the
// context passed to fn, and commits or rolls back on return — repository
// methods take a transaction from the context when one is present and
// use the pool otherwise, so the same method serves both transactional
// and standalone callers with no signature change.
type Transactor interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
