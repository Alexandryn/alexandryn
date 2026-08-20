package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// TruncateTables empties every named table and cascades to dependent
// rows, restarting identity sequences — backend-test-harness.md FR-3's
// prescribed isolation mechanism: a test-level teardown, not an outer
// transaction rolled back at the end. An outer transaction doesn't
// compose with a repository method that opens and commits its own
// internal transaction against the shared pool (backend-persistence.md
// FR-4) — the two would not share a connection or transaction context.
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
