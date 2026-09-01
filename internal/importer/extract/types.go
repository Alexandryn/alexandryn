package extract

import (
	"errors"
	"strings"
	"unicode"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// Format is the content-sniffed file format (backend-file-extractors.md FR-2).
type Format string

const (
	FormatEPUB    Format = "epub"
	FormatPDF     Format = "pdf"
	FormatCBZ     Format = "cbz"
	FormatUnknown Format = "unknown"
)

// Standard extraction errors (backend-file-extractors.md Failure modes table).
var (
	ErrOversized      = errors.New("file or decompressed content exceeds size limit")
	ErrTooManyEntries = errors.New("zip archive exceeds 10,000 entry limit")
	ErrMalformed      = errors.New("malformed or unparseable container")
	ErrNoTitle        = errors.New("file contains no title metadata")
)

// Bounds per backend-file-extractors.md FR-1, FR-3, FR-4, FR-5.
const (
	MaxRawSpoolBytes         = 250 * 1024 * 1024 // 250 MiB raw byte limit
	MaxDecompressedReadBytes = 200 * 1024 * 1024 // 200 MiB total decompressed limit
	MaxZipEntryCount         = 10000             // 10,000 entries max in zip archive
	MaxCoverBytes            = 10 * 1024 * 1024  // 10 MiB cover limit
	MaxTextLength            = 255
	MaxDescriptionLength     = 10000
)

// ExtractedMetadata is the shared DTO for extracted book metadata (backend-file-extractors.md FR-4).
type ExtractedMetadata struct {
	Title       string   `json:"title"`
	Authors     []string `json:"authors"`
	ISBN        *string  `json:"isbn,omitempty"`
	Language    *string  `json:"language,omitempty"`
	Publisher   *string  `json:"publisher,omitempty"`
	Description *string  `json:"description,omitempty"`
	CoverBytes  []byte   `json:"coverBytes,omitempty"`
	Format      Format   `json:"format"`
}

// NewExtractedMetadata constructs and sanitizes an ExtractedMetadata DTO.
// Fields failing validation (control characters, oversized, invalid ISBN/language)
// are dropped/sanitized gracefully per FR-4, while empty/whitespace-only title returns ErrNoTitle (FR-8).
func NewExtractedMetadata(
	rawTitle string,
	rawAuthors []string,
	rawISBN *string,
	rawLanguage *string,
	rawPublisher *string,
	rawDescription *string,
	coverBytes []byte,
	format Format,
) (ExtractedMetadata, error) {
	title := strings.TrimSpace(rawTitle)
	if title == "" {
		return ExtractedMetadata{}, ErrNoTitle
	}
	title = sanitizeField(title, MaxTextLength)
	if title == "" {
		return ExtractedMetadata{}, ErrNoTitle
	}

	var authors []string
	for _, a := range rawAuthors {
		clean := sanitizeField(strings.TrimSpace(a), MaxTextLength)
		if clean != "" {
			authors = append(authors, clean)
		}
	}
	if authors == nil {
		authors = []string{}
	}

	var isbn *string
	if rawISBN != nil {
		cleanISBN := strings.TrimSpace(*rawISBN)
		if err := domain.ValidateISBN(cleanISBN); err == nil {
			isbn = &cleanISBN
		}
	}

	var language *string
	if rawLanguage != nil {
		cleanLang := strings.TrimSpace(*rawLanguage)
		if err := domain.ValidateLanguageTag(cleanLang); err == nil {
			language = &cleanLang
		}
	}

	var publisher *string
	if rawPublisher != nil {
		cleanPub := sanitizeField(strings.TrimSpace(*rawPublisher), MaxTextLength)
		if cleanPub != "" {
			publisher = &cleanPub
		}
	}

	var description *string
	if rawDescription != nil {
		cleanDesc := sanitizeField(strings.TrimSpace(*rawDescription), MaxDescriptionLength)
		if cleanDesc != "" {
			description = &cleanDesc
		}
	}

	var cover []byte
	if len(coverBytes) > 0 && len(coverBytes) <= MaxCoverBytes {
		cover = coverBytes
	}

	return ExtractedMetadata{
		Title:       title,
		Authors:     authors,
		ISBN:        isbn,
		Language:    language,
		Publisher:   publisher,
		Description: description,
		CoverBytes:  cover,
		Format:      format,
	}, nil
}

func sanitizeField(s string, maxLen int) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsControl(r) || r == '\n' || r == '\r' || r == '\t' {
			b.WriteRune(r)
		}
	}
	res := strings.TrimSpace(b.String())
	runes := []rune(res)
	if len(runes) > maxLen {
		res = string(runes[:maxLen])
	}
	return strings.TrimSpace(res)
}
