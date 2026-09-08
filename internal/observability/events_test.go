package observability_test

import (
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/observability"
)

func TestNewSystemEvent_SanitizesPayload(t *testing.T) {
	ev := observability.NewSystemEvent{
		EventKind: "test.event",
		Payload: map[string]any{
			"validKey": 123,
			"password": "secret-password",
			"token":    "jwt.token.here",
			"position": "epubcfi(/6/4)",
		},
		RetentionDays: 30,
	}

	sanitized := ev.SanitizedPayload()
	if _, ok := sanitized["password"]; ok {
		t.Errorf("payload contained password key")
	}
	if _, ok := sanitized["token"]; ok {
		t.Errorf("payload contained token key")
	}
	if _, ok := sanitized["position"]; ok {
		t.Errorf("payload contained position key")
	}
	if val, ok := sanitized["validKey"]; !ok || val != 123 {
		t.Errorf("validKey missing or wrong: %v", val)
	}
}

// audit 0016 #296: nested maps/slices and the "note" key must be
// sanitized too — reading position lives under a nested "detail" object
// and highlight text lives under "note".
func TestNewSystemEvent_SanitizesNestedPayload(t *testing.T) {
	ev := observability.NewSystemEvent{
		EventKind: "reading.progress",
		Payload: map[string]any{
			"workId": "w1",
			"detail": map[string]any{
				"cfi":        "epubcfi(/6/4!/10)",
				"percentage": 55,
				"safe":       "keep",
			},
			"note": "the highlighted passage text",
			"events": []any{
				map[string]any{"token": "leak", "kind": "ok"},
			},
		},
		RetentionDays: 30,
	}

	s := ev.SanitizedPayload()
	if _, ok := s["note"]; ok {
		t.Error("top-level note not stripped")
	}
	detail, ok := s["detail"].(map[string]any)
	if !ok {
		t.Fatalf("detail missing or wrong type: %T", s["detail"])
	}
	if _, ok := detail["cfi"]; ok {
		t.Error("nested cfi not stripped")
	}
	if _, ok := detail["percentage"]; ok {
		t.Error("nested percentage not stripped")
	}
	if detail["safe"] != "keep" {
		t.Errorf("nested safe key lost: %v", detail["safe"])
	}
	evs, ok := s["events"].([]any)
	if !ok || len(evs) != 1 {
		t.Fatalf("events slice missing: %v", s["events"])
	}
	if _, ok := evs[0].(map[string]any)["token"]; ok {
		t.Error("token inside a slice element not stripped")
	}
}

func TestRetention_PurgeAtCalculatedCorrectly(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	ev := observability.NewSystemEvent{
		EventKind:     "test.event",
		RetentionDays: 14,
	}

	purgeAt := ev.CalculatePurgeAt(now)
	expected := now.Add(14 * 24 * time.Hour)
	if !purgeAt.Equal(expected) {
		t.Errorf("purgeAt = %v, want %v", purgeAt, expected)
	}

	// Default fallback to 30 days if <= 0
	evDefault := observability.NewSystemEvent{
		EventKind:     "test.event",
		RetentionDays: 0,
	}
	purgeAtDefault := evDefault.CalculatePurgeAt(now)
	expectedDefault := now.Add(30 * 24 * time.Hour)
	if !purgeAtDefault.Equal(expectedDefault) {
		t.Errorf("default purgeAt = %v, want %v", purgeAtDefault, expectedDefault)
	}
}
