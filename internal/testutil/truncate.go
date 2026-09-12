package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// TruncateTables empties every named table and cascades to dependent
// rows, restarting identity sequences. This provides test-level teardown
// isolation rather than an outer transaction rolled back at the end,
// because outer transactions do not compose with repository methods that
// open and commit their own internal transactions against the shared pool.
// Call it via t.Cleanup so teardown runs whether or not the test itself
// failed.
func TruncateTables(ctx context.Context, db *sql.DB, tables ...string) error {
	if len(tables) == 0 {
		return nil
	}
	quoted := make([]string, len(tables))
	for i, name := range tables {
		quoted[i] = `"` + name + `"`
	}
	_, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(quoted, ", ")))
	return err
}
