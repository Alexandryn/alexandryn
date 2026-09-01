// Package api holds the reading API's pure validation and the progress
// reconcile-and-persist orchestration (backend-reading-api.md). HTTP
// handlers in internal/transport/http compose these; nothing here
// imports net/http.
package api

import (
	"regexp"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// maxCFILength bounds a stored CFI string — a real EPUB CFI is short
// (tens of characters); anything far longer is hostile or malformed
// (backend-reading-api.md FR-4).
const maxCFILength = 1024

// cfiBody is the character set the CFI grammar's step/offset/assertion
// syntax can produce: digits, '/', ':', '.', '!', '~', '@', ',', '[',
// ']', '(', ')', '^', ';', '=', '-', '+', '*', and letters/spaces for
// assertion text. This is a plausibility check, not a grammar — the
// roadmap chose to lean on foliate-js's epubcfi.js for real
// generation/resolution (FR-4).
var cfiBody = regexp.MustCompile(`^[A-Za-z0-9/:.!~@,\[\]()^;=+*_\- ]*$`)

// ValidateCFI is FR-4's shallow structural check: begins with the
// literal "epubcfi(", ends with ")", brackets and parentheses balanced,
// and contains only characters the CFI grammar permits. It deliberately
// does NOT verify the string is a semantically valid CFI a resolver
// could act on.
func ValidateCFI(cfi string) error {
	if len(cfi) == 0 {
		return &domain.Error{Category: domain.InvalidInput, Message: "position is empty"}
	}
	if len(cfi) > maxCFILength {
		return &domain.Error{Category: domain.InvalidInput, Message: "position is too long to be a valid CFI"}
	}
	if !strings.HasPrefix(cfi, "epubcfi(") || !strings.HasSuffix(cfi, ")") {
		return &domain.Error{Category: domain.InvalidInput, Message: "position is not an epubcfi(...) value"}
	}
	inner := cfi[len("epubcfi(") : len(cfi)-1]
	if !cfiBody.MatchString(inner) {
		return &domain.Error{Category: domain.InvalidInput, Message: "position contains characters the CFI grammar cannot produce"}
	}
	if !balanced(cfi) {
		return &domain.Error{Category: domain.InvalidInput, Message: "position has unbalanced brackets or parentheses"}
	}
	return nil
}

func balanced(s string) bool {
	var round, square int
	for _, r := range s {
		switch r {
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		}
		if round < 0 || square < 0 {
			return false
		}
	}
	return round == 0 && square == 0
}

// CFISortsBefore reports whether a sorts strictly before b under the CFI
// specification's own defined ordering. Both must already be
// structurally valid (ValidateCFI). The comparison is a
// segment-by-segment numeric compare of the step/offset integers — the
// same lexical-numeric order foliate-js's epubcfi.compare uses for the
// common case, sufficient for FR-7's "endCfi must not sort before
// startCfi" boundary check.
func CFISortsBefore(a, b string) bool {
	return compareCFI(a, b) < 0
}

var cfiNum = regexp.MustCompile(`\d+`)

func compareCFI(a, b string) int {
	as := cfiNum.FindAllString(a, -1)
	bs := cfiNum.FindAllString(b, -1)
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, y := atoiPad(as[i]), atoiPad(bs[i])
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	default:
		return 0
	}
}

func atoiPad(s string) int64 {
	var n int64
	for _, r := range s {
		n = n*10 + int64(r-'0')
	}
	return n
}

var uuidV4 = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-4[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

// ValidateDeviceID checks the X-Device-Id header is a UUID v4 in shape —
// never for authenticity, which is phase 12's concern
// (backend-reading-api.md FR-1).
func ValidateDeviceID(v string) error {
	if !uuidV4.MatchString(strings.TrimSpace(v)) {
		return &domain.Error{Category: domain.InvalidInput, Message: "X-Device-Id must be a version 4 UUID"}
	}
	return nil
}

// maxObservedEpoch bounds the reported epoch (FR-3) — the real epoch
// only ever increments on a deliberate override, so a very large value
// is malformed input, not a legitimate state.
const maxObservedEpoch = 1 << 40

// ValidateObservedEpoch checks the reported epoch is a non-negative
// integer within a sane bound (FR-3). A value above the stored epoch is
// not an error here — it is clamped at reconcile time.
func ValidateObservedEpoch(epoch int64) error {
	if epoch < 0 || epoch > maxObservedEpoch {
		return &domain.Error{Category: domain.InvalidInput, Message: "observedEpoch is out of range"}
	}
	return nil
}

// ValidatePercentage restates domain.NewPercentage's [0.0, 1.0] invariant
// at the transport boundary (FR-3).
func ValidatePercentage(p float64) error {
	if p < 0.0 || p > 1.0 {
		return &domain.Error{Category: domain.InvalidInput, Message: "percentage must be within [0.0, 1.0]"}
	}
	return nil
}
