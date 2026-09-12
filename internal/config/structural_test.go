package config

import "testing"

// TestFieldTable_Categories verifies that every configuration key is assigned
// exactly one category (required, optional-default, or optional-no-default).
func TestFieldTable_Categories(t *testing.T) {
	want := map[string]category{
		"DATABASE_URL":                   categoryOptionalNoDefault,
		"LOG_LEVEL":                      categoryOptionalDefault,
		"SHUTDOWN_GRACE_PERIOD":          categoryOptionalDefault,
		"DB_POOL_MAX_CONNS":              categoryOptionalDefault,
		"HTTP_MAX_BODY_BYTES":            categoryOptionalDefault,
		"HTTP_READ_TIMEOUT":              categoryOptionalDefault,
		"HTTP_WRITE_TIMEOUT":             categoryOptionalDefault,
		"HTTP_IDLE_TIMEOUT":              categoryOptionalDefault,
		"OPEN_LIBRARY_USER_AGENT":        categoryRequired,
		"BIND_ADDRESS":                   categoryOptionalDefault,
		"TLS_CERT_FILE":                  categoryOptionalNoDefault,
		"TLS_KEY_FILE":                   categoryOptionalNoDefault,
		"DESKTOP_PARENT_PID":             categoryOptionalNoDefault,
		"ACME_ENABLED":                   categoryOptionalDefault,
		"ACME_DOMAIN":                    categoryOptionalNoDefault,
		"ACME_EMAIL":                     categoryOptionalNoDefault,
		"ACME_CACHE_DIR":                 categoryOptionalNoDefault,
		"CORS_ALLOWED_ORIGINS":           categoryOptionalNoDefault,
		"TRUSTED_PROXY_CIDRS":            categoryOptionalNoDefault,
		"DEVICE_PAIRING_SECRET":          categoryOptionalNoDefault,
		"SOURCE_ALLOW_PRIVATE_ADDRESSES": categoryOptionalDefault,
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
			t.Errorf("key %s category = %v, want %v", key, gotCat, wantCat)
		}
	}
	for key := range got {
		if _, expected := want[key]; !expected {
			t.Errorf("key %s in the field table but not in expected set", key)
		}
	}
}
