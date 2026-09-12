// Package idgen provides the production implementation of
// domain.IDGenerator: RFC 4122 UUID v4, generated via crypto/rand
// using the standard library.
package idgen

import (
	"crypto/rand"
	"fmt"
)

// Generator produces RFC 4122 UUID v4 values. The zero value is ready to
// use; New exists for symmetry with this project's other constructors,
// not because there's any state to initialize.
type Generator struct{}

func New() *Generator { return &Generator{} }

// NewID returns a new UUID v4 string. crypto/rand.Read failing indicates
// a broken OS entropy source — exceptionally rare, and not a condition
// any caller could meaningfully recover from — so this panics rather
// than returning an error, matching domain.IDGenerator's own no-error
// signature and testutil.FakeIDGenerator's existing precedent of
// panicking on a broken invariant rather than returning a zero value
// that would silently corrupt whatever it's used to identify.
func (g *Generator) NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("idgen: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10 (RFC 4122)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
