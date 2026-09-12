// Package pairing holds cryptographic adapters for device pairing, including
// CSPRNG-backed pairing code generation and encryption/blind-indexing helpers.
// Pure state machine logic and domain value objects reside in internal/domain.
package pairing

import (
	"crypto/rand"
	"fmt"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// codeBytes is 5, ensuring 8 Crockford-base32 characters carry exactly 40 bits of entropy.
const codeBytes = 5

// GeneratePairingCode reads codeBytes from crypto/rand, encodes them as 8
// Crockford-base32 characters, and constructs a domain.PairingCode.
// Read errors are returned directly and never retried with pseudo-random sources.
func GeneratePairingCode() (domain.PairingCode, error) {
	var b [codeBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		return domain.PairingCode{}, fmt.Errorf("pairing code entropy: %w", err)
	}
	return domain.NewPairingCode(encodeCrockford40(b))
}

// encodeCrockford40 packs 5 bytes (40 bits) big-endian into 8 5-bit
// groups, mapped through domain.CrockfordAlphabet.
func encodeCrockford40(b [codeBytes]byte) string {
	v := uint64(b[0])<<32 | uint64(b[1])<<24 | uint64(b[2])<<16 | uint64(b[3])<<8 | uint64(b[4])
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = domain.CrockfordAlphabet[v&0x1f]
		v >>= 5
	}
	return string(out)
}
