package importer

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

type MatchCandidateType string

const (
	MatchCandidateTypeExistingEdition MatchCandidateType = "existing_edition"
	MatchCandidateTypeOpenLibraryWork MatchCandidateType = "open_library_work"
)

type MatchConfidence string

const (
	MatchConfidenceExact  MatchConfidence = "exact"
	MatchConfidenceHigh   MatchConfidence = "high"
	MatchConfidenceMedium MatchConfidence = "medium"
	MatchConfidenceLow    MatchConfidence = "low"
)

// MatchCandidate represents a suggested bibliographic match.
type MatchCandidate struct {
	Type               MatchCandidateType `json:"type"`
	Confidence         MatchConfidence    `json:"confidence"`
	Title              string             `json:"title"`
	Author             *string            `json:"author"`
	CoverURL           *string            `json:"coverUrl"`
	EditionID          *string            `json:"editionId,omitempty"`
	OpenLibraryWorkKey *string            `json:"openLibraryWorkKey,omitempty"`
}

// ExistingLibraryFinder searches the existing owned library for matching editions.
type ExistingLibraryFinder interface {
	FindOwnedByISBN(ctx context.Context, isbn string) ([]postgres.ExistingEditionHit, error)
}

// OpenLibrarySearcher is the metadata search capability for Open Library.
type OpenLibrarySearcher interface {
	Search(ctx context.Context, q string, limit, offset int) (*openlibrary.NormalisedSearchResponse, error)
}

// Matcher evaluates extracted metadata against existing library records and Open Library.
type Matcher struct {
	library     ExistingLibraryFinder
	openLibrary OpenLibrarySearcher
}

// NewMatcher creates a new Matcher.
func NewMatcher(lib ExistingLibraryFinder, ol OpenLibrarySearcher) *Matcher {
	return &Matcher{
		library:     lib,
		openLibrary: ol,
	}
}

var whitespaceRegex = regexp.MustCompile(`\s+`)

func normalizeText(s string) string {
	lower := strings.ToLower(s)
	collapsed := whitespaceRegex.ReplaceAllString(lower, " ")
	return strings.TrimSpace(collapsed)
}

// Match runs both existing-library lookup and Open Library search, returning
// candidates and whether the auto-accept condition is met.
func (m *Matcher) Match(ctx context.Context, meta extract.ExtractedMetadata) ([]MatchCandidate, bool, error) {
	var candidates []MatchCandidate
	var exactHitsCount int

	// 1. Existing library lookup
	if meta.ISBN != nil && *meta.ISBN != "" && m.library != nil {
		hits, err := m.library.FindOwnedByISBN(ctx, *meta.ISBN)
		if err == nil {
			exactHitsCount = len(hits)
			for _, hit := range hits {
				edID := hit.EditionID
				var authorPtr *string
				if hit.Author != "" {
					a := hit.Author
					authorPtr = &a
				}
				candidates = append(candidates, MatchCandidate{
					Type:       MatchCandidateTypeExistingEdition,
					Confidence: MatchConfidenceExact,
					Title:      hit.Title,
					Author:     authorPtr,
					CoverURL:   hit.CoverURL,
					EditionID:  &edID,
				})
			}
		}
	}

	// 2. Open Library search
	if m.openLibrary != nil && meta.Title != "" {
		query := meta.Title
		if len(meta.Authors) > 0 {
			query = fmt.Sprintf("%s %s", meta.Title, meta.Authors[0])
		}

		res, err := m.openLibrary.Search(ctx, query, 5, 0)
		if err == nil && res != nil {
			for _, item := range res.Items {
				conf := m.ScoreOpenLibraryResult(meta, item)
				workKey := item.OpenLibraryWorkKey
				var authorPtr *string
				if len(item.Authors) > 0 {
					a := item.Authors[0].Name
					authorPtr = &a
				}
				candidates = append(candidates, MatchCandidate{
					Type:               MatchCandidateTypeOpenLibraryWork,
					Confidence:         conf,
					Title:              item.Title,
					Author:             authorPtr,
					CoverURL:           item.CoverURL,
					OpenLibraryWorkKey: &workKey,
				})
			}
		}
	}

	// Auto-accept rule: exactly ONE exact-confidence existing-library match.
	autoAccept := (exactHitsCount == 1)

	return candidates, autoAccept, nil
}

// ScoreOpenLibraryResult computes confidence for an Open Library result.
func (m *Matcher) ScoreOpenLibraryResult(meta extract.ExtractedMetadata, item openlibrary.NormalisedSearchResult) MatchConfidence {
	metaTitle := normalizeText(meta.Title)
	itemTitle := normalizeText(item.Title)

	// Check if parenthetical qualifier is present on one but not other ("Series" vs "Series (Color)")
	hasParenDiff := (strings.Contains(meta.Title, "(") != strings.Contains(item.Title, "("))

	var authorMatches bool
	for _, metaAuthor := range meta.Authors {
		normMetaA := normalizeText(metaAuthor)
		for _, itemA := range item.Authors {
			normItemA := normalizeText(itemA.Name)
			if normMetaA != "" && (normMetaA == normItemA || strings.Contains(normItemA, normMetaA) || strings.Contains(normMetaA, normItemA)) {
				authorMatches = true
				break
			}
		}
		if authorMatches {
			break
		}
	}

	if metaTitle == itemTitle {
		if authorMatches {
			if hasParenDiff {
				return MatchConfidenceMedium
			}
			return MatchConfidenceHigh
		}
		return MatchConfidenceMedium
	}

	// Title difference only in parenthetical
	baseMeta := normalizeText(strings.Split(meta.Title, "(")[0])
	baseItem := normalizeText(strings.Split(item.Title, "(")[0])
	if baseMeta != "" && baseMeta == baseItem {
		return MatchConfidenceMedium
	}

	return MatchConfidenceLow
}
