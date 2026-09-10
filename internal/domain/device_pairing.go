package domain

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Device pairing (domain-device-pairing.md). A pairing is a thin
// host-initiated bootstrap that proves "the host operator intends this
// device" and carries the host address to it; the device still
// authenticates with a phase-12 account afterwards (ADR 0028 §6). These
// are pure types — no networking, no crypto beyond a constant-time
// compare, no persistence. Random-byte generation lives in an adapter
// (backend-network-transport.md FR-9); the domain takes the generated
// value.
//
// Sentinel errors: unlike the rest of internal/domain, this file exports
// errors.Is targets. domain-device-pairing.md's Failure-modes table
// names them (ErrPairingExpired, ErrPairingWrongState, …) and its
// acceptance criteria require an illegal transition to return "the right
// typed error" — a caller (and a test) must be able to tell expiry from
// a wrong-state move from a code mismatch without matching on a message
// string. Each is still wrapped in *Error for its Category.
var (
	ErrInvalidPairingCode   = errors.New("pairing code is malformed")
	ErrPairingExpired       = errors.New("pairing session has expired")
	ErrPairingWrongState    = errors.New("pairing session is in the wrong state for this operation")
	ErrPairingCodeMismatch  = errors.New("pairing code does not match")
	ErrDeviceRevoked        = errors.New("paired device is revoked")
	ErrDeviceAlreadyRevoked = errors.New("paired device is already revoked")
)

// CrockfordAlphabet is the Crockford base32 alphabet (uppercase, no
// padding), excluding I, L, O and U. domain-device-pairing.md FR-2
// rejects a non-Crockford character rather than leniently decoding
// I->1 / O->0. Exported so the one other place that needs it —
// internal/pairing's crypto/rand-backed generator, which must encode
// into exactly the alphabet this package's constructor validates against
// — references this single copy instead of keeping its own.
const CrockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// pairingCodeLen is the fixed shape: eight Crockford characters, the
// smallest fixed format that can carry the >= 40 bits of entropy
// GeneratePairingCode reads from crypto/rand (backend-network-transport.md
// FR-9 — that entropy floor is the generator's acceptance criterion, not
// this constructor's, which can only check shape).
const pairingCodeLen = 8

// PairingCode is a value object wrapping a normalized code. The wrapped
// value is a []byte, not a string, deliberately: it makes the type
// non-comparable, so `==` on two PairingCode values is a compile error
// and Equal (constant-time) is the only comparison path (FR-1).
type PairingCode struct {
	b []byte
}

// NewPairingCode constructs a PairingCode from an untrusted string. It
// case-folds, strips the display hyphen, and rejects anything that is
// not exactly pairingCodeLen Crockford characters (FR-1/FR-2).
func NewPairingCode(untrusted string) (PairingCode, error) {
	n := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(untrusted), "-", ""))
	if len(n) != pairingCodeLen {
		return PairingCode{}, &Error{
			Category: InvalidInput,
			Message:  fmt.Sprintf("pairing code must be %d characters", pairingCodeLen),
			Err:      ErrInvalidPairingCode,
		}
	}
	for _, r := range n {
		if !strings.ContainsRune(CrockfordAlphabet, r) {
			return PairingCode{}, &Error{
				Category: InvalidInput,
				Message:  "pairing code must use the Crockford base32 alphabet (no I, L, O or U)",
				Err:      ErrInvalidPairingCode,
			}
		}
	}
	return PairingCode{b: []byte(n)}, nil
}

// Equal reports whether other holds the same normalized value, compared
// in constant time (FR-1 — a non-constant-time compare of a 40-bit
// secret over a LAN is a realistic side channel). A zero-value
// PairingCode never equals anything, itself included: subtle.ConstantTimeCompare
// treats nil==nil as a match, which on the credential path must not read
// as "the code is correct".
func (c PairingCode) Equal(other PairingCode) bool {
	if len(c.b) != pairingCodeLen || len(other.b) != pairingCodeLen {
		return false
	}
	return subtle.ConstantTimeCompare(c.b, other.b) == 1
}

// Normalized returns the 8-character uppercase form — for the persistence
// layer to encrypt (backend-network-api.md FR-7) and the API to return in
// the initiate response. Callers MUST NOT log it (Observability NFR).
func (c PairingCode) Normalized() string { return string(c.b) }

// Display returns the grouped XXXX-XXXX form for a UI or a QR payload.
func (c PairingCode) Display() string {
	if len(c.b) != pairingCodeLen {
		return ""
	}
	return string(c.b[:4]) + "-" + string(c.b[4:])
}

// String is deliberately redacted: a PairingCode is a short-lived
// credential-equivalent and must not leak through an accidental %v or a
// structured log of a containing value (constitution §8). Use Normalized
// or Display explicitly.
func (c PairingCode) String() string { return "[pairing code]" }

// PairingState is the pairing session lifecycle (FR-4). consumed and
// expired are terminal.
type PairingState string

const (
	PairingPending  PairingState = "pending"
	PairingVerified PairingState = "verified"
	PairingConsumed PairingState = "consumed"
	PairingExpired  PairingState = "expired"
)

func (s PairingState) valid() bool {
	switch s {
	case PairingPending, PairingVerified, PairingConsumed, PairingExpired:
		return true
	default:
		return false
	}
}

// maxPairingTTL is the ceiling enforced at construction (FR-3), not left
// to the caller — a future endpoint cannot widen the guessing window by
// passing a larger ttl. Shortening it is allowed.
const maxPairingTTL = 5 * time.Minute

// PairingSession is the aggregate owning one pairing attempt. Every
// legal advance is a method; an illegal move is a returned typed error,
// never a silent mutation.
type PairingSession struct {
	id          PairingSessionID
	initiatedBy UserID
	code        PairingCode
	state       PairingState
	createdAt   time.Time
	expiresAt   time.Time
	deviceID    *DeviceID
}

// NewPairingSession constructs a fresh pending session (FR-3). ttl must
// be > 0 and <= maxPairingTTL.
func NewPairingSession(id PairingSessionID, initiatedBy UserID, code PairingCode, ttl time.Duration, now time.Time) (*PairingSession, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "pairing session ID cannot be empty"}
	}
	if strings.TrimSpace(string(initiatedBy)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "pairing session must record the initiating user"}
	}
	if ttl <= 0 || ttl > maxPairingTTL {
		return nil, &Error{Category: InvalidInput, Message: fmt.Sprintf("pairing TTL must be within (0, %s]", maxPairingTTL)}
	}
	return &PairingSession{
		id:          id,
		initiatedBy: initiatedBy,
		code:        code,
		state:       PairingPending,
		createdAt:   now,
		expiresAt:   now.Add(ttl),
	}, nil
}

// RehydratePairingSession reconstructs a session from a persisted row.
// Unlike the RehydrateWork family it re-validates every invariant
// (domain-device-pairing.md Reliability NFR: "a row that violates an
// invariant MUST fail re-hydration loudly, not load a malformed
// aggregate"). The PairingCode is handed in already reconstructed and
// validated by NewPairingCode in the repository (ADR 0028 §6 — codes are
// stored encrypted, not hashed, precisely so this is possible).
func RehydratePairingSession(
	id PairingSessionID,
	initiatedBy UserID,
	code PairingCode,
	state PairingState,
	createdAt, expiresAt time.Time,
	deviceID *DeviceID,
) (*PairingSession, error) {
	if strings.TrimSpace(string(id)) == "" || strings.TrimSpace(string(initiatedBy)) == "" {
		return nil, &Error{Category: Internal, Message: "persisted pairing session is missing an identifier"}
	}
	if len(code.b) != pairingCodeLen {
		return nil, &Error{Category: Internal, Message: "persisted pairing session has an invalid code"}
	}
	if !state.valid() {
		return nil, &Error{Category: Internal, Message: fmt.Sprintf("persisted pairing session has an unknown state %q", state)}
	}
	window := expiresAt.Sub(createdAt)
	if window <= 0 || window > maxPairingTTL {
		return nil, &Error{Category: Internal, Message: fmt.Sprintf("persisted pairing session has an out-of-range TTL of %s", window)}
	}
	if deviceID != nil && strings.TrimSpace(string(*deviceID)) == "" {
		return nil, &Error{Category: Internal, Message: "persisted pairing session has a blank device ID"}
	}
	// state and deviceID must agree: Verify sets a non-nil DeviceID
	// exactly when it advances to verified, and consumed comes only from
	// verified. A row where they disagree is a malformed aggregate
	// (Reliability NFR). expired is unconstrained — a session can expire
	// from pending (no device) or verified (a device).
	switch state {
	case PairingPending:
		if deviceID != nil {
			return nil, &Error{Category: Internal, Message: "persisted pending pairing session has a device ID"}
		}
	case PairingVerified, PairingConsumed:
		if deviceID == nil {
			return nil, &Error{Category: Internal, Message: fmt.Sprintf("persisted %s pairing session has no device ID", state)}
		}
	}
	return &PairingSession{
		id:          id,
		initiatedBy: initiatedBy,
		code:        code,
		state:       state,
		createdAt:   createdAt,
		expiresAt:   expiresAt,
		deviceID:    deviceID,
	}, nil
}

func (s *PairingSession) ID() PairingSessionID { return s.id }
func (s *PairingSession) InitiatedBy() UserID  { return s.initiatedBy }
func (s *PairingSession) Code() PairingCode    { return s.code }
func (s *PairingSession) State() PairingState  { return s.state }
func (s *PairingSession) CreatedAt() time.Time { return s.createdAt }
func (s *PairingSession) ExpiresAt() time.Time { return s.expiresAt }

// DeviceID is nil until Verify sets it.
func (s *PairingSession) DeviceID() *DeviceID { return s.deviceID }

func (s *PairingSession) isExpired(now time.Time) bool {
	return !now.Before(s.expiresAt)
}

// terminal reports whether the session can never change state again.
// consumed and expired are terminal (FR-4); no method, expiry included,
// touches a terminal session.
func (s *PairingSession) terminal() bool {
	return s.state == PairingConsumed || s.state == PairingExpired
}

// ExpireAt moves a pending or verified session to expired when now is at
// or after ExpiresAt (FR-5). It is a no-op — no error, no mutation —
// otherwise or when the session is already terminal.
func (s *PairingSession) ExpireAt(now time.Time) {
	if s.state != PairingPending && s.state != PairingVerified {
		return
	}
	if s.isExpired(now) {
		s.state = PairingExpired
	}
}

// checkAdvanceable is Verify and Consume's shared preamble: reject a
// terminal session outright with no mutation (FR-4), expire a stale
// pending/verified session (FR-5 — this is the one legal mutation a
// rejected call can still make), then reject if the session isn't in the
// one state the caller expects. expiredMsg differs per caller: Verify
// passes the same generic "not recognised" text a wrong code returns (no
// oracle on the unauthenticated pairing route); Consume names the expiry
// plainly (its caller is already authenticated).
func (s *PairingSession) checkAdvanceable(now time.Time, expected PairingState, verb, expiredMsg string) error {
	if s.terminal() {
		return &Error{
			Category: Conflict,
			Message:  fmt.Sprintf("pairing session in state %q cannot be %s", s.state, verb),
			Err:      ErrPairingWrongState,
		}
	}
	if s.isExpired(now) {
		s.state = PairingExpired
		return &Error{Category: Conflict, Message: expiredMsg, Err: ErrPairingExpired}
	}
	if s.state != expected {
		return &Error{
			Category: Conflict,
			Message:  fmt.Sprintf("pairing session in state %q cannot be %s", s.state, verb),
			Err:      ErrPairingWrongState,
		}
	}
	return nil
}

// Verify checks the shared preamble (FR-4/FR-5), then the caller's
// device ID, then the submitted code in constant time (FR-6). A wrong
// code returns ErrPairingCodeMismatch and does NOT change state —
// burning the session on a wrong guess would let an unauthenticated LAN
// client grief a real pairing. On a match it sets the DeviceID and moves
// to verified.
func (s *PairingSession) Verify(now time.Time, submitted PairingCode, deviceID DeviceID) error {
	if err := s.checkAdvanceable(now, PairingPending, "verified", "pairing code not recognised"); err != nil {
		return err
	}
	// The device ID is the caller's (a fresh server-generated value), not
	// the attacker's — check it before the code compare so a bad-argument
	// call never runs the comparison at all.
	if strings.TrimSpace(string(deviceID)) == "" {
		return &Error{Category: Internal, Message: "Verify requires a non-empty device ID"}
	}
	if !s.code.Equal(submitted) {
		return &Error{Category: Conflict, Message: "pairing code not recognised", Err: ErrPairingCodeMismatch}
	}
	d := deviceID
	s.deviceID = &d
	s.state = PairingVerified
	return nil
}

// Consume moves a verified session to consumed (FR-7) — the point at
// which the enrolment grant has been issued to the device. Expressible
// as a single call so the repository can run it inside the same
// transaction that writes the PairedDevice row (ADR 0021).
func (s *PairingSession) Consume(now time.Time) error {
	if err := s.checkAdvanceable(now, PairingVerified, "consumed", "pairing session has expired"); err != nil {
		return err
	}
	s.state = PairingConsumed
	return nil
}

// DeviceClass is a coarse classification the API layer supplies (FR-8),
// derived outside this package from a request header. The domain stores
// the enum, never parses a header — no raw client identifier enters the
// pairing types (FR-10).
type DeviceClass string

const (
	DeviceClassPhone   DeviceClass = "phone"
	DeviceClassTablet  DeviceClass = "tablet"
	DeviceClassDesktop DeviceClass = "desktop"
	DeviceClassTV      DeviceClass = "tv"
	DeviceClassUnknown DeviceClass = "unknown"
)

func (c DeviceClass) valid() bool {
	switch c {
	case DeviceClassPhone, DeviceClassTablet, DeviceClassDesktop, DeviceClassTV, DeviceClassUnknown:
		return true
	default:
		return false
	}
}

// EnrolledVia records how a device joined (FR-8). Phase 13 only ever
// writes EnrolledViaPairingCode; EnrolledViaPasswordLogin is reserved for
// phase 14's device inventory so the persisted check constraint does not
// need a later migration.
type EnrolledVia string

const (
	EnrolledViaPairingCode   EnrolledVia = "pairing_code"
	EnrolledViaPasswordLogin EnrolledVia = "password_login"
)

func (v EnrolledVia) valid() bool {
	return v == EnrolledViaPairingCode || v == EnrolledViaPasswordLogin
}

const maxDeviceLabelRunes = 100

// PairedDevice records a completed enrolment (FR-8). Owner is the user
// who completed POST /api/v1/auth/login carrying the enrolment grant —
// NOT the admin who initiated the PairingSession (ADR 0028 §6).
type PairedDevice struct {
	id           DeviceID
	owner        UserID
	label        string
	deviceClass  DeviceClass
	enrolledVia  EnrolledVia
	createdAt    time.Time
	lastSeenAt   time.Time
	revokedAt    *time.Time
	syncCursor   int64
	lastSyncedAt *time.Time
}

// NewPairedDevice constructs a device owned by a known user. label is
// trimmed; deviceClass and enrolledVia must be known values.
func NewPairedDevice(id DeviceID, owner UserID, label string, deviceClass DeviceClass, enrolledVia EnrolledVia, now time.Time) (*PairedDevice, error) {
	if strings.TrimSpace(string(id)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "device ID cannot be empty"}
	}
	if strings.TrimSpace(string(owner)) == "" {
		return nil, &Error{Category: InvalidInput, Message: "paired device must have an owner"}
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, &Error{Category: InvalidInput, Message: "device label cannot be empty"}
	}
	if len([]rune(label)) > maxDeviceLabelRunes {
		return nil, &Error{Category: InvalidInput, Message: fmt.Sprintf("device label cannot exceed %d characters", maxDeviceLabelRunes)}
	}
	if !deviceClass.valid() {
		return nil, &Error{Category: InvalidInput, Message: fmt.Sprintf("unknown device class %q", deviceClass)}
	}
	if !enrolledVia.valid() {
		return nil, &Error{Category: InvalidInput, Message: fmt.Sprintf("unknown enrolment method %q", enrolledVia)}
	}
	return &PairedDevice{
		id:          id,
		owner:       owner,
		label:       label,
		deviceClass: deviceClass,
		enrolledVia: enrolledVia,
		createdAt:   now,
		lastSeenAt:  now,
		syncCursor:  0,
	}, nil
}

// RehydratePairedDevice reconstructs a device from a persisted row,
// re-validating every invariant (Reliability NFR).
func RehydratePairedDevice(
	id DeviceID,
	owner UserID,
	label string,
	deviceClass DeviceClass,
	enrolledVia EnrolledVia,
	createdAt, lastSeenAt time.Time,
	revokedAt *time.Time,
	syncCursor int64,
	lastSyncedAt *time.Time,
) (*PairedDevice, error) {
	if strings.TrimSpace(string(id)) == "" || strings.TrimSpace(string(owner)) == "" {
		return nil, &Error{Category: Internal, Message: "persisted paired device is missing an identifier"}
	}
	if strings.TrimSpace(label) == "" || len([]rune(label)) > maxDeviceLabelRunes {
		return nil, &Error{Category: Internal, Message: "persisted paired device has an invalid label"}
	}
	if !deviceClass.valid() {
		return nil, &Error{Category: Internal, Message: fmt.Sprintf("persisted paired device has an unknown class %q", deviceClass)}
	}
	if !enrolledVia.valid() {
		return nil, &Error{Category: Internal, Message: fmt.Sprintf("persisted paired device has an unknown enrolment method %q", enrolledVia)}
	}
	if syncCursor < 0 {
		return nil, &Error{Category: Internal, Message: "persisted paired device has a negative sync cursor"}
	}
	return &PairedDevice{
		id:           id,
		owner:        owner,
		label:        label,
		deviceClass:  deviceClass,
		enrolledVia:  enrolledVia,
		createdAt:    createdAt,
		lastSeenAt:   lastSeenAt,
		revokedAt:    revokedAt,
		syncCursor:   syncCursor,
		lastSyncedAt: lastSyncedAt,
	}, nil
}

func (d *PairedDevice) ID() DeviceID             { return d.id }
func (d *PairedDevice) Owner() UserID            { return d.owner }
func (d *PairedDevice) Label() string            { return d.label }
func (d *PairedDevice) DeviceClass() DeviceClass { return d.deviceClass }
func (d *PairedDevice) EnrolledVia() EnrolledVia { return d.enrolledVia }
func (d *PairedDevice) CreatedAt() time.Time     { return d.createdAt }
func (d *PairedDevice) LastSeenAt() time.Time    { return d.lastSeenAt }
func (d *PairedDevice) SyncCursor() int64        { return d.syncCursor }
func (d *PairedDevice) LastSyncedAt() *time.Time { return d.lastSyncedAt }

// RevokedAt is nil for an active device.
func (d *PairedDevice) RevokedAt() *time.Time { return d.revokedAt }

// Revoke marks the device revoked (FR-9). A double-revoke is a caller
// bug worth surfacing — idempotent in effect but returned as an error.
func (d *PairedDevice) Revoke(now time.Time) error {
	if d.revokedAt != nil {
		return &Error{Category: Conflict, Message: "paired device is already revoked", Err: ErrDeviceAlreadyRevoked}
	}
	t := now
	d.revokedAt = &t
	return nil
}

// Touch advances LastSeenAt forward only, never backward under clock
// skew, and fails on a revoked device (FR-9). No phase-13 caller — kept
// for phase 14's inventory and covered by the phase-13 tests.
func (d *PairedDevice) Touch(now time.Time) error {
	if d.revokedAt != nil {
		return &Error{Category: Conflict, Message: "paired device is revoked", Err: ErrDeviceRevoked}
	}
	if now.After(d.lastSeenAt) {
		d.lastSeenAt = now
	}
	return nil
}

// AdvanceCursor advances the sync cursor to a strictly higher value and records the sync timestamp (FR-5).
// Fails if the device is revoked or if newCursor <= current.
func (d *PairedDevice) AdvanceCursor(newCursor int64, at time.Time) error {
	if d.revokedAt != nil {
		return &Error{Category: Conflict, Message: "paired device is revoked", Err: ErrDeviceRevoked}
	}
	if newCursor <= d.syncCursor {
		return &Error{Category: InvalidInput, Message: fmt.Sprintf("new sync cursor %d must be strictly greater than current %d", newCursor, d.syncCursor)}
	}
	d.syncCursor = newCursor
	t := at
	d.lastSyncedAt = &t
	return nil
}
