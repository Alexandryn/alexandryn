package domain

// ReadingPreferences is scoped per DeviceID, not global and not
// per-Work. Settings is an opaque key-value map representing device-specific
// display settings (font, theme, spacing, etc.). No cross-device
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
