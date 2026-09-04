// Package pairing holds the crypto-touching adapters for device pairing
// (backend-network-transport.md FR-9, backend-network-api.md FR-7): the
// pairing-code generator here, and — from Tier 3 — the AES-256-GCM
// encrypt / HMAC blind-index used to persist a code. The pure state
// machine and value objects live in internal/domain; this package never
// imports transport or persistence.
package pairing

import (
	"crypto/rand"
	"fmt"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// crockford is the Crockford base32 alphabet, uppercase, no padding,
// excluding I, L, O, U — the same set domain.NewPairingCode accepts.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// codeBytes is 5, so 8 Crockford characters carry exactly 40 bits of
// entropy — the floor backend-network-transport.md FR-9 requires against
// a 5-minute, rate-limited online guess.
const codeBytes = 5

// GeneratePairingCode reads codeBytes from crypto/rand, encodes them as 8
// Crockford-base32 characters, and returns the value through domain's
// validated constructor (domain-device-pairing.md FR-1/FR-2). A
// crypto/rand read error is returned, never swallowed and never retried
// with a weaker source — this file imports no math/rand.
func GeneratePairingCode() (domain.PairingCode, error) {
	var b [codeBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return domain.PairingCode{}, fmt.Errorf("pairing code entropy: %w", err)
	}
	return domain.NewPairingCode(encodeCrockford40(b))
}

// encodeCrockford40 packs 5 bytes (40 bits) big-endian into 8 5-bit
// groups, each mapped through the Crockford alphabet.
func encodeCrockford40(b [codeBytes]byte) string {
	v := uint64(b[0])<<32 | uint64(b[1])<<24 | uint64(b[2])<<16 | uint64(b[3])<<8 | uint64(b[4])
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = crockford[v&0x1f]
		v >>= 5
	}
	return string(out)
}
