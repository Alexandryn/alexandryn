package domain

// ReadingPreferences (FR-5) is scoped per DeviceID, not global and not
// per-Work. Settings is opaque beyond validation — phase 11 defines the
// actual field set (font, theme, spacing, etc.); a fresh DeviceID starts
// with an empty map, representing "system defaults" abstractly since
// this phase doesn't define what those defaults are. No cross-device
// inheritance: each instance is independent, never copied from another.
type ReadingPreferences struct {
	deviceID DeviceID
	settings map[string]string
}

func NewReadingPreferences(deviceID DeviceID) *ReadingPreferences {
	return &ReadingPreferences{deviceID: deviceID, settings: map[string]string{}}
}

func (p *ReadingPreferences) DeviceID() DeviceID { return p.deviceID }

func (p *ReadingPreferences) Settings() map[string]string { return p.settings }

func (p *ReadingPreferences) Set(key, value string) { p.settings[key] = value }
