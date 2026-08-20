package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ValidateBoundedText rejects a string that is empty, whitespace-only,
// exceeds maxLen runes, or contains a control character. Every free-form
// field across all five phase 02 specs (title, subtitle, publisher,
// author name, subject, collection name, source label, bookmark label,
// highlight note) routes through this one implementation rather than
// repeating the same checks per type (E2, tasks/plan-phase02-domain.md).
//
// An empty or whitespace-only value fails identically to one exceeding
// maxLen — a maximum-length bound alone admits an all-whitespace string
// that satisfies it while carrying no content, which is a real recurring
// shape of upstream metadata (review 0049 finding 5,
// domain-bibliographic.md FR-6).
func ValidateBoundedText(field, s string, maxLen int) error {
	if strings.TrimSpace(s) == "" {
		return &Error{Category: InvalidInput, Message: fmt.Sprintf("%s must not be empty or whitespace-only", field)}
	}
	if n := len([]rune(s)); n > maxLen {
		return &Error{Category: InvalidInput, Message: fmt.Sprintf("%s exceeds maximum length of %d characters", field, maxLen)}
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return &Error{Category: InvalidInput, Message: fmt.Sprintf("%s contains a control character", field)}
		}
	}
	return nil
}

// bcp47Pattern is a structural check, not full IANA subtag-registry
// validation: a primary subtag of 2-8 letters, followed by any number of
// 1-8 character alphanumeric subtags separated by hyphens. This is
// deliberately narrower than validating against the actual registry (no
// dependency needed for it) — domain-bibliographic.md FR-5 requires "a
// BCP-47 tag, not arbitrary text," not registry membership.
var bcp47Pattern = regexp.MustCompile(`^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$`)

// ValidateLanguageTag rejects a string that doesn't have BCP-47's basic
// shape (domain-bibliographic.md FR-5).
func ValidateLanguageTag(s string) error {
	if !bcp47Pattern.MatchString(s) {
		return &Error{Category: InvalidInput, Message: "language must be a valid BCP-47 tag"}
	}
	return nil
}

// ValidateISBN rejects a string that is not a checksum-valid ISBN-10 or
// ISBN-13, hyphens ignored (domain-bibliographic.md FR-2). Checksum
// validation, not just length/format — resolves that spec's own Open
// questions item ("ISBN-10 vs. ISBN-13 checksum validation, or just
// format/length... a phase 03/07 implementation detail") in favor of the
// stronger check: the algorithm is well-defined, cheap, and needs no
// external dependency.
func ValidateISBN(s string) error {
	cleaned := strings.ReplaceAll(s, "-", "")
	valid := false
	switch len(cleaned) {
	case 10:
		valid = isValidISBN10(cleaned)
	case 13:
		valid = isValidISBN13(cleaned)
	}
	if !valid {
		return &Error{Category: InvalidInput, Message: "isbn is not a valid ISBN-10 or ISBN-13"}
	}
	return nil
}

func isValidISBN10(s string) bool {
	sum := 0
	for i := 0; i < 10; i++ {
		c := s[i]
		var v int
		switch {
		case c >= '0' && c <= '9':
			v = int(c - '0')
		case c == 'X' && i == 9:
			v = 10
		default:
			return false
		}
		sum += v * (10 - i)
	}
	return sum%11 == 0
}

func isValidISBN13(s string) bool {
	sum := 0
	for i := 0; i < 13; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
		v := int(c - '0')
		if i%2 == 0 {
			sum += v
		} else {
			sum += v * 3
		}
	}
	return sum%10 == 0
}
