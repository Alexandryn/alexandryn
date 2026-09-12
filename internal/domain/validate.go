package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ValidateBoundedText rejects a string that is empty, whitespace-only,
// exceeds maxLen runes, or contains a control character. Free-form
// fields across domain models (title, subtitle, publisher, author name,
// subject, collection name, source label, bookmark label, highlight note)
// route through this shared validation logic.
//
// An empty or whitespace-only value fails identically to one exceeding
// maxLen, preventing whitespace-padded empty strings from passing.
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

// bcp47Pattern performs a structural check: a primary subtag of 2-8 letters,
// followed by any number of 1-8 character alphanumeric subtags separated by hyphens.
var bcp47Pattern = regexp.MustCompile(`^[a-zA-Z]{2,8}(-[a-zA-Z0-9]{1,8})*$`)

// ValidateLanguageTag validates that a string adheres to standard BCP-47 tag structure.
func ValidateLanguageTag(s string) error {
	if !bcp47Pattern.MatchString(s) {
		return &Error{Category: InvalidInput, Message: "language must be a valid BCP-47 tag"}
	}
	return nil
}

// ValidateISBN rejects a string that is not a checksum-valid ISBN-10 or
// ISBN-13, ignoring hyphens.
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
