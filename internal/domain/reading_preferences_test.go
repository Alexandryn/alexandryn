package domain_test

import "testing"

import "github.com/Alexandryn/alexandryn/internal/domain"

// domain-reading.md FR-5: ReadingPreferences MUST be scoped per DeviceID.
// A new DeviceID's first ReadingPreferences MUST start from system
// defaults — there is no cross-device inheritance.
func TestNewReadingPreferences_StartsEmpty(t *testing.T) {
	prefs := domain.NewReadingPreferences("device-1")
	if prefs.DeviceID() != "device-1" {
		t.Fatalf("DeviceID() = %v, want device-1", prefs.DeviceID())
	}
	if len(prefs.Settings()) != 0 {
		t.Fatalf("Settings() = %v, want empty (system defaults, nothing set yet)", prefs.Settings())
	}
}

func TestReadingPreferences_SetAndGet(t *testing.T) {
	prefs := domain.NewReadingPreferences("device-1")
	prefs.Set("theme", "dark")

	if got := prefs.Settings()["theme"]; got != "dark" {
		t.Fatalf("Settings()[\"theme\"] = %q, want %q", got, "dark")
	}
}

// A second device never inherits a first device's settings — each
// ReadingPreferences instance is independent.
func TestReadingPreferences_NoCrossDeviceInheritance(t *testing.T) {
	first := domain.NewReadingPreferences("device-1")
	first.Set("theme", "dark")

	second := domain.NewReadingPreferences("device-2")
	if len(second.Settings()) != 0 {
		t.Fatalf("a fresh device's Settings() = %v, want empty, not inherited from another device", second.Settings())
	}
}
