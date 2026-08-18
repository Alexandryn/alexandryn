package testutil_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func TestIntegrationTestMain_UnsetFailsLoudWithoutRunningTests(t *testing.T) {
	lookup := func(string) (string, bool) { return "", false }
	ran := false
	run := func() int { ran = true; return 0 }
	var out bytes.Buffer

	code := testutil.IntegrationTestMain(lookup, run, &out)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero when TEST_DATABASE_URL is unset")
	}
	if ran {
		t.Fatal("run() was called even though TEST_DATABASE_URL was unset")
	}
	if !strings.Contains(out.String(), "TEST_DATABASE_URL") {
		t.Fatalf("message doesn't name the missing variable: %q", out.String())
	}
}

func TestIntegrationTestMain_EmptyFailsLoudTheSameWayAsUnset(t *testing.T) {
	lookup := func(string) (string, bool) { return "", true } // present, but empty
	ran := false
	run := func() int { ran = true; return 0 }
	var out bytes.Buffer

	code := testutil.IntegrationTestMain(lookup, run, &out)

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero when TEST_DATABASE_URL is empty")
	}
	if ran {
		t.Fatal("run() was called even though TEST_DATABASE_URL was empty")
	}
	if !strings.Contains(out.String(), "TEST_DATABASE_URL") {
		t.Fatalf("message doesn't name the missing variable: %q", out.String())
	}
}

func TestIntegrationTestMain_SetRunsAndPassesThroughExitCode(t *testing.T) {
	lookup := func(string) (string, bool) { return "postgres://fixture/proof", true }
	ran := false
	run := func() int { ran = true; return 7 }
	var out bytes.Buffer

	code := testutil.IntegrationTestMain(lookup, run, &out)

	if !ran {
		t.Fatal("run() was not called even though TEST_DATABASE_URL was set")
	}
	if code != 7 {
		t.Fatalf("exit code = %d, want 7 (run()'s own return value, passed through)", code)
	}
}
