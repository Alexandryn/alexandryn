package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Phase 13 (domain-device-pairing.md). Pure types, injected clock, no
// I/O. Tests that MUST fail before implementation: the full PairingSession
// legal-path test, the illegal-transition table, and the PairingCode
// constant-time-Equal structural test.

// --- T1.1 PairingCode (FR-1/FR-2) ---

func mustCode(t *testing.T, s string) domain.PairingCode {
	t.Helper()
	c, err := domain.NewPairingCode(s)
	if err != nil {
		t.Fatalf("NewPairingCode(%q): %v", s, err)
	}
	return c
}

func TestPairingCode_ConstructorAcceptsExactlyTheCrockfordShape(t *testing.T) {
	ok := []string{
		"ABCD2345",  // bare 8
		"abcd2345",  // case-folded
		"ABCD-2345", // display hyphen stripped
		"abcd-2345",
		"0123456789ABCDEFGHJKMNPQRSTVWXYZ"[:8], // first 8 of the alphabet
	}
	for _, s := range ok {
		if _, err := domain.NewPairingCode(s); err != nil {
			t.Errorf("NewPairingCode(%q) = %v, want ok", s, err)
		}
	}

	bad := map[string]string{
		"seven c":     "ABC2345",
		"nine chars":  "ABCD23456",
		"letter I":    "ABCDI345",
		"letter L":    "ABCDL345",
		"letter O":    "ABCDO345",
		"letter U":    "ABCDU345",
		"padding =":   "ABCD234=",
		"whitespace":  "ABCD 345",
		"punctuation": "ABCD_345",
		"empty":       "",
	}
	for name, s := range bad {
		t.Run(name, func(t *testing.T) {
			if _, err := domain.NewPairingCode(s); err == nil {
				t.Fatalf("NewPairingCode(%q) = nil, want an error", s)
			} else if domain.CategoryOf(err) != domain.InvalidInput {
				t.Fatalf("category = %v, want InvalidInput", domain.CategoryOf(err))
			}
		})
	}
}

func TestPairingCode_EqualIsConstantTimeAndNormalizes(t *testing.T) {
	a := mustCode(t, "ABCD-2345")
	b := mustCode(t, "abcd2345") // same value, different spelling
	c := mustCode(t, "ZZZZ2345")

	if !a.Equal(b) {
		t.Fatal("Equal should be true for the same normalized value")
	}
	if a.Equal(c) {
		t.Fatal("Equal should be false for different values")
	}
	// The type must not be == comparable — the only equality path is
	// Equal (FR-1). This is enforced structurally: a non-comparable
	// field makes == a compile error. See device_pairing_audit_test.go.
}

func TestPairingCode_DisplayAndRedaction(t *testing.T) {
	c := mustCode(t, "abcd2345")
	if c.Normalized() != "ABCD2345" {
		t.Fatalf("Normalized() = %q, want ABCD2345", c.Normalized())
	}
	if c.Display() != "ABCD-2345" {
		t.Fatalf("Display() = %q, want ABCD-2345", c.Display())
	}
	// String() must not leak the code (constitution §8 — it is a
	// short-lived credential-equivalent).
	if strings.Contains(c.String(), "ABCD") || strings.Contains(c.String(), "2345") {
		t.Fatalf("String() leaked the code: %q", c.String())
	}
}

// --- T1.2 PairingSession (FR-3..FR-7) ---

const testTTL = 5 * time.Minute

func mustSession(t *testing.T, now time.Time) *domain.PairingSession {
	t.Helper()
	s, err := domain.NewPairingSession("ps-1", "admin-1", mustCode(t, "ABCD2345"), testTTL, now)
	if err != nil {
		t.Fatalf("NewPairingSession: %v", err)
	}
	return s
}

func TestPairingSession_Construction(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	s := mustSession(t, now)
	if s.State() != domain.PairingPending {
		t.Fatalf("state = %v, want pending", s.State())
	}
	if !s.ExpiresAt().Equal(now.Add(testTTL)) {
		t.Fatalf("ExpiresAt = %v, want %v", s.ExpiresAt(), now.Add(testTTL))
	}
	if s.DeviceID() != nil {
		t.Fatal("DeviceID should be nil at construction")
	}

	for _, ttl := range []time.Duration{0, -time.Second, 5*time.Minute + time.Nanosecond, time.Hour} {
		if _, err := domain.NewPairingSession("ps-1", "admin-1", mustCode(t, "ABCD2345"), ttl, now); err == nil {
			t.Fatalf("NewPairingSession ttl=%v = nil, want an error", ttl)
		}
	}
	for _, ttl := range []time.Duration{time.Second, 5 * time.Minute} {
		if _, err := domain.NewPairingSession("ps-1", "admin-1", mustCode(t, "ABCD2345"), ttl, now); err != nil {
			t.Fatalf("NewPairingSession ttl=%v = %v, want ok", ttl, err)
		}
	}
	if _, err := domain.NewPairingSession("", "admin-1", mustCode(t, "ABCD2345"), testTTL, now); err == nil {
		t.Fatal("empty id should be rejected")
	}
	if _, err := domain.NewPairingSession("ps-1", "", mustCode(t, "ABCD2345"), testTTL, now); err == nil {
		t.Fatal("empty initiatedBy should be rejected")
	}
}

func TestPairingSession_LegalPath(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	s := mustSession(t, now)

	if err := s.Verify(now.Add(time.Minute), mustCode(t, "ABCD2345"), "dev-1"); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if s.State() != domain.PairingVerified {
		t.Fatalf("state = %v, want verified", s.State())
	}
	if s.DeviceID() == nil || *s.DeviceID() != "dev-1" {
		t.Fatalf("DeviceID = %v, want dev-1", s.DeviceID())
	}
	if err := s.Consume(now.Add(2 * time.Minute)); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if s.State() != domain.PairingConsumed {
		t.Fatalf("state = %v, want consumed", s.State())
	}
}

func TestPairingSession_WrongCodeDoesNotBurnTheSession(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	s := mustSession(t, now)

	err := s.Verify(now.Add(time.Minute), mustCode(t, "ZZZZ2345"), "dev-1")
	if !errors.Is(err, domain.ErrPairingCodeMismatch) {
		t.Fatalf("err = %v, want ErrPairingCodeMismatch", err)
	}
	if s.State() != domain.PairingPending {
		t.Fatalf("state = %v, want still pending after a wrong guess", s.State())
	}
	// a subsequent correct Verify still works
	if err := s.Verify(now.Add(2*time.Minute), mustCode(t, "ABCD2345"), "dev-1"); err != nil {
		t.Fatalf("Verify after a wrong guess: %v", err)
	}
}

func TestPairingSession_IllegalTransitions(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	code := mustCode(t, "ABCD2345")

	t.Run("Consume before Verify", func(t *testing.T) {
		s := mustSession(t, now)
		if err := s.Consume(now.Add(time.Minute)); !errors.Is(err, domain.ErrPairingWrongState) {
			t.Fatalf("err = %v, want ErrPairingWrongState", err)
		}
		if s.State() != domain.PairingPending {
			t.Fatalf("state mutated to %v", s.State())
		}
	})

	t.Run("Verify twice", func(t *testing.T) {
		s := mustSession(t, now)
		_ = s.Verify(now.Add(time.Minute), code, "dev-1")
		if err := s.Verify(now.Add(2*time.Minute), code, "dev-2"); !errors.Is(err, domain.ErrPairingWrongState) {
			t.Fatalf("err = %v, want ErrPairingWrongState", err)
		}
	})

	t.Run("Consume twice", func(t *testing.T) {
		s := mustSession(t, now)
		_ = s.Verify(now.Add(time.Minute), code, "dev-1")
		_ = s.Consume(now.Add(2 * time.Minute))
		if err := s.Consume(now.Add(3 * time.Minute)); !errors.Is(err, domain.ErrPairingWrongState) {
			t.Fatalf("err = %v, want ErrPairingWrongState", err)
		}
	})

	t.Run("Verify with zero DeviceID", func(t *testing.T) {
		s := mustSession(t, now)
		if err := s.Verify(now.Add(time.Minute), code, ""); err == nil {
			t.Fatal("a zero DeviceID must be rejected")
		}
		if s.State() != domain.PairingPending {
			t.Fatal("state mutated on a bad-argument call")
		}
	})

	t.Run("a terminal session past its TTL is not silently re-expired", func(t *testing.T) {
		// Regression: Verify/Consume must check terminal state before
		// expiry, so a consumed session called again after the deadline
		// returns ErrPairingWrongState (no mutation), not ErrPairingExpired.
		s := mustSession(t, now)
		_ = s.Verify(now.Add(time.Minute), code, "dev-1")
		_ = s.Consume(now.Add(2 * time.Minute))
		past := s.ExpiresAt().Add(time.Hour)

		if err := s.Verify(past, code, "dev-2"); !errors.Is(err, domain.ErrPairingWrongState) {
			t.Fatalf("Verify on a consumed+stale session: err = %v, want ErrPairingWrongState", err)
		}
		if err := s.Consume(past); !errors.Is(err, domain.ErrPairingWrongState) {
			t.Fatalf("Consume on a consumed+stale session: err = %v, want ErrPairingWrongState", err)
		}
		if s.State() != domain.PairingConsumed {
			t.Fatalf("state = %v, want still consumed", s.State())
		}
	})
}

func TestPairingSession_Expiry(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	code := mustCode(t, "ABCD2345")

	t.Run("ExpireAt is a no-op before the deadline and idempotent after", func(t *testing.T) {
		s := mustSession(t, now)
		s.ExpireAt(s.ExpiresAt().Add(-time.Nanosecond))
		if s.State() != domain.PairingPending {
			t.Fatal("expired one tick early")
		}
		s.ExpireAt(s.ExpiresAt())
		if s.State() != domain.PairingExpired {
			t.Fatal("did not expire at the deadline")
		}
		s.ExpireAt(s.ExpiresAt().Add(time.Hour)) // idempotent, no panic
		if s.State() != domain.PairingExpired {
			t.Fatal("state changed after terminal")
		}
	})

	t.Run("Verify rejects a stale session without a prior ExpireAt", func(t *testing.T) {
		s := mustSession(t, now)
		if err := s.Verify(s.ExpiresAt(), code, "dev-1"); !errors.Is(err, domain.ErrPairingExpired) {
			t.Fatalf("err = %v, want ErrPairingExpired", err)
		}
		if s.State() != domain.PairingExpired {
			t.Fatal("Verify should have moved a stale session to expired")
		}
	})

	t.Run("Consume rejects a stale session without a prior ExpireAt", func(t *testing.T) {
		s := mustSession(t, now)
		_ = s.Verify(now.Add(time.Minute), code, "dev-1")
		if err := s.Consume(s.ExpiresAt().Add(time.Second)); !errors.Is(err, domain.ErrPairingExpired) {
			t.Fatalf("err = %v, want ErrPairingExpired", err)
		}
	})
}

// --- T1.3 PairedDevice (FR-8/FR-9) ---

func mustDevice(t *testing.T, now time.Time) *domain.PairedDevice {
	t.Helper()
	d, err := domain.NewPairedDevice("dev-1", "user-1", "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now)
	if err != nil {
		t.Fatalf("NewPairedDevice: %v", err)
	}
	return d
}

func TestPairedDevice_Construction(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	d := mustDevice(t, now)
	if d.Owner() != "user-1" || d.DeviceClass() != domain.DeviceClassPhone || d.EnrolledVia() != domain.EnrolledViaPairingCode {
		t.Fatalf("device fields not set: %+v", d)
	}
	if !d.CreatedAt().Equal(now) || !d.LastSeenAt().Equal(now) {
		t.Fatal("CreatedAt/LastSeenAt should both be now at construction")
	}
	if d.RevokedAt() != nil {
		t.Fatal("RevokedAt should be nil at construction")
	}

	if _, err := domain.NewPairedDevice("", "user-1", "x", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now); err == nil {
		t.Fatal("empty id rejected")
	}
	if _, err := domain.NewPairedDevice("dev-1", "", "x", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now); err == nil {
		t.Fatal("empty owner rejected")
	}
	if _, err := domain.NewPairedDevice("dev-1", "user-1", "   ", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now); err == nil {
		t.Fatal("blank label rejected")
	}
	if _, err := domain.NewPairedDevice("dev-1", "user-1", strings.Repeat("x", 101), domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now); err == nil {
		t.Fatal("over-long label rejected")
	}
	if _, err := domain.NewPairedDevice("dev-1", "user-1", "x", "martian", domain.EnrolledViaPairingCode, now); err == nil {
		t.Fatal("unknown device class rejected")
	}
	if _, err := domain.NewPairedDevice("dev-1", "user-1", "x", domain.DeviceClassPhone, "telepathy", now); err == nil {
		t.Fatal("unknown enrolment method rejected")
	}
}

func TestPairedDevice_RevokeAndTouch(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	t.Run("Touch moves LastSeenAt forward only", func(t *testing.T) {
		d := mustDevice(t, now)
		if err := d.Touch(now.Add(time.Hour)); err != nil {
			t.Fatalf("Touch: %v", err)
		}
		if !d.LastSeenAt().Equal(now.Add(time.Hour)) {
			t.Fatalf("LastSeenAt = %v, want +1h", d.LastSeenAt())
		}
		_ = d.Touch(now) // backward — ignored
		if !d.LastSeenAt().Equal(now.Add(time.Hour)) {
			t.Fatal("Touch moved LastSeenAt backward")
		}
	})

	t.Run("Revoke is idempotent-error; a revoked device is inert", func(t *testing.T) {
		d := mustDevice(t, now)
		if err := d.Revoke(now.Add(time.Minute)); err != nil {
			t.Fatalf("Revoke: %v", err)
		}
		if d.RevokedAt() == nil || !d.RevokedAt().Equal(now.Add(time.Minute)) {
			t.Fatalf("RevokedAt = %v", d.RevokedAt())
		}
		if err := d.Revoke(now.Add(2 * time.Minute)); !errors.Is(err, domain.ErrDeviceAlreadyRevoked) {
			t.Fatalf("second Revoke err = %v, want ErrDeviceAlreadyRevoked", err)
		}
		if err := d.Touch(now.Add(3 * time.Minute)); !errors.Is(err, domain.ErrDeviceRevoked) {
			t.Fatalf("Touch on a revoked device err = %v, want ErrDeviceRevoked", err)
		}
	})
}

// --- T1.4 rehydration (FR-3 Reliability NFR) ---

func TestRehydratePairingSession_RevalidatesInvariants(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	code := mustCode(t, "ABCD2345")

	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingPending, now, now.Add(testTTL), nil); err != nil {
		t.Fatalf("valid row: %v", err)
	}
	// ExpiresAt more than 5m after CreatedAt
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingPending, now, now.Add(time.Hour), nil); err == nil {
		t.Fatal("a >5m window must fail rehydration")
	}
	// unknown state string
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingState("weird"), now, now.Add(testTTL), nil); err == nil {
		t.Fatal("an unknown state must fail rehydration")
	}
	// state and deviceID must agree
	dev := domain.DeviceID("dev-1")
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingPending, now, now.Add(testTTL), &dev); err == nil {
		t.Fatal("a pending session with a device ID must fail rehydration")
	}
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingVerified, now, now.Add(testTTL), nil); err == nil {
		t.Fatal("a verified session with no device ID must fail rehydration")
	}
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingConsumed, now, now.Add(testTTL), nil); err == nil {
		t.Fatal("a consumed session with no device ID must fail rehydration")
	}
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingVerified, now, now.Add(testTTL), &dev); err != nil {
		t.Fatalf("a verified session with a device ID is valid: %v", err)
	}
	// expired is unconstrained
	if _, err := domain.RehydratePairingSession("ps-1", "admin-1", code, domain.PairingExpired, now, now.Add(testTTL), nil); err != nil {
		t.Fatalf("an expired session with no device ID is valid: %v", err)
	}
}

func TestRehydratePairedDevice_RevalidatesInvariants(t *testing.T) {
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)

	if _, err := domain.RehydratePairedDevice("dev-1", "user-1", "Pixel 8", domain.DeviceClassPhone, domain.EnrolledViaPairingCode, now, now, nil); err != nil {
		t.Fatalf("valid row: %v", err)
	}
	if _, err := domain.RehydratePairedDevice("dev-1", "user-1", "x", "martian", domain.EnrolledViaPairingCode, now, now, nil); err == nil {
		t.Fatal("unknown device class must fail rehydration")
	}
	if _, err := domain.RehydratePairedDevice("dev-1", "user-1", "x", domain.DeviceClassPhone, "telepathy", now, now, nil); err == nil {
		t.Fatal("unknown enrolment method must fail rehydration")
	}
}
