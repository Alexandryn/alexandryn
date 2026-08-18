package config

import "testing"

// FR-3 structural check: every key in FR-4's table must carry exactly one
// of the three categories, matching backend-configuration.md's own table.
// A key drifting into a different category than the spec declares — the
// exact defect review 0022 found once (DATABASE_URL described as
// "required in dev") — fails this test immediately, without depending on
// someone remembering to add a matching pair of behavioral tests by hand.
func TestFieldTable_CategoriesMatchTheSpec(t *testing.T) {
	want := map[string]category{
		"DATABASE_URL":            categoryOptionalNoDefault,
		"LOG_LEVEL":               categoryOptionalDefault,
		"SHUTDOWN_GRACE_PERIOD":   categoryOptionalDefault,
		"DB_POOL_MAX_CONNS":       categoryOptionalDefault,
		"HTTP_MAX_BODY_BYTES":     categoryOptionalDefault,
		"HTTP_READ_TIMEOUT":       categoryOptionalDefault,
		"HTTP_WRITE_TIMEOUT":      categoryOptionalDefault,
		"HTTP_IDLE_TIMEOUT":       categoryOptionalDefault,
		"OPEN_LIBRARY_USER_AGENT": categoryRequired,
		"BIND_ADDRESS":            categoryOptionalDefault,
		"TLS_CERT_FILE":           categoryOptionalNoDefault,
		"TLS_KEY_FILE":            categoryOptionalNoDefault,
	}

	got := map[string]category{}
	for _, f := range fields {
		if _, dup := got[f.key]; dup {
			t.Fatalf("key %s appears more than once in the field table", f.key)
		}
		got[f.key] = f.category
	}

	for key, wantCat := range want {
		gotCat, ok := got[key]
		if !ok {
			t.Errorf("key %s missing from the field table", key)
			continue
		}
		if gotCat != wantCat {
			t.Errorf("key %s category = %v, want %v (backend-configuration.md FR-4)", key, gotCat, wantCat)
		}
	}
	for key := range got {
		if _, expected := want[key]; !expected {
			t.Errorf("key %s in the field table but not in this task's expected set — update this test's want map", key)
		}
	}
}
