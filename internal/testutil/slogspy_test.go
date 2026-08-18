package testutil_test

import (
	"log/slog"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/testutil"
)

func TestSpyHandler_CapturesMessageLevelAndAttrs(t *testing.T) {
	spy := testutil.NewSpyHandler()
	logger := slog.New(spy)

	logger.Info("request completed", "status", 200, "path", "/healthz")

	records := spy.Records()
	if len(records) != 1 {
		t.Fatalf("Records() len = %d, want 1", len(records))
	}

	r := records[0]
	if r.Message != "request completed" {
		t.Fatalf("Message = %q, want %q", r.Message, "request completed")
	}
	if r.Level != slog.LevelInfo {
		t.Fatalf("Level = %v, want %v", r.Level, slog.LevelInfo)
	}

	attrs := map[string]slog.Value{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value
		return true
	})
	if got := attrs["status"].Int64(); got != 200 {
		t.Fatalf(`attr "status" = %d, want 200`, got)
	}
	if got := attrs["path"].String(); got != "/healthz" {
		t.Fatalf(`attr "path" = %q, want "/healthz"`, got)
	}
}

func TestSpyHandler_WithAttrsCarriesBoundFieldsIntoCapturedRecord(t *testing.T) {
	spy := testutil.NewSpyHandler()
	logger := slog.New(spy).With("request_id", "abc-123")

	logger.Warn("slow query")

	records := spy.Records()
	if len(records) != 1 {
		t.Fatalf("Records() len = %d, want 1", len(records))
	}

	found := false
	records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "request_id" && a.Value.String() == "abc-123" {
			found = true
		}
		return true
	})
	if !found {
		t.Fatal(`bound attr "request_id"="abc-123" missing from captured record`)
	}
}

func TestSpyHandler_WithGroupNestsBoundAttrs(t *testing.T) {
	spy := testutil.NewSpyHandler()
	logger := slog.New(spy).WithGroup("request").With("id", "abc-123")

	logger.Info("handled")

	records := spy.Records()
	if len(records) != 1 {
		t.Fatalf("Records() len = %d, want 1", len(records))
	}

	var group slog.Value
	found := false
	records[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "request" {
			group = a.Value
			found = true
		}
		return true
	})
	if !found {
		t.Fatal(`captured record missing the "request" group`)
	}

	inner := false
	for _, a := range group.Group() {
		if a.Key == "id" && a.Value.String() == "abc-123" {
			inner = true
		}
	}
	if !inner {
		t.Fatal(`"request" group missing "id"="abc-123"`)
	}
}

func TestSpyHandler_ContainsChecksMessageAndAttrValues(t *testing.T) {
	spy := testutil.NewSpyHandler()
	logger := slog.New(spy)

	logger.Info("config loaded", "source", "postgres://user:s3cr3t@host/db")

	if !spy.Contains("s3cr3t") {
		t.Fatal("Contains() = false, want true — the secret is in a captured attr value")
	}
	if spy.Contains("never-logged") {
		t.Fatal("Contains() = true for a string that was never logged")
	}
}
