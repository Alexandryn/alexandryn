package domain_test

import "testing"

import "github.com/Alexandryn/alexandryn/internal/domain"

// domain-reading.md's own risk table: "progress never exceeds its
// bounds."
func TestNewPercentage(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		wantErr bool
	}{
		{"zero is legal", 0.0, false},
		{"one is legal", 1.0, false},
		{"midpoint is legal", 0.5, false},
		{"negative rejected", -0.1, true},
		{"over one rejected", 1.1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.NewPercentage(tt.value)
			if tt.wantErr && err == nil {
				t.Fatalf("NewPercentage(%v) = nil error, want an error", tt.value)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("NewPercentage(%v) = %v, want nil", tt.value, err)
			}
		})
	}
}

// FR-8: a reading status MUST be computed from Percentage, never stored
// separately.
func TestPercentage_Status(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  domain.ReadingStatus
	}{
		{"zero is NotStarted", 0.0, domain.NotStarted},
		{"one is Finished", 1.0, domain.Finished},
		{"anything between is InProgress", 0.42, domain.InProgress},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := domain.NewPercentage(tt.value)
			if err != nil {
				t.Fatalf("NewPercentage: %v", err)
			}
			if got := p.Status(); got != tt.want {
				t.Fatalf("Status() = %v, want %v", got, tt.want)
			}
		})
	}
}
