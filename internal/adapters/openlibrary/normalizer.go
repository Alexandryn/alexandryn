package openlibrary

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

var openLibraryWorkIDRegex = regexp.MustCompile(`^OL[0-9]+W$`)
var openLibraryAuthorIDRegex = regexp.MustCompile(`^OL[0-9]+A$`)
var openLibraryEditionIDRegex = regexp.MustCompile(`^OL[0-9]+M$`)

// IsValidWorkKey checks if an ID matches Open Library work key shape (e.g. OL82563W).
func IsValidWorkKey(id string) bool {
	return openLibraryWorkIDRegex.MatchString(id)
}

// CleanKey removes common Open Library path prefixes like /works/, /books/, /authors/.
func CleanKey(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "/works/")
	raw = strings.TrimPrefix(raw, "/books/")
	raw = strings.TrimPrefix(raw, "/authors/")
	raw = strings.TrimPrefix(raw, "/languages/")
	return raw
}

func containsDisallowedControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return true
		}
	}
	return false
}

func validateTitle(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len([]rune(trimmed)) > 500 || containsDisallowedControlChars(trimmed) {
		return "", false
	}
	return trimmed, true
}

func validateSubtitle(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if len([]rune(trimmed)) > 500 || containsDisallowedControlChars(trimmed) {
		return ""
	}
	return trimmed
}

func extractDescription(raw any) string {
	if raw == nil {
		return ""
	}
	var text string
	switch v := raw.(type) {
	case string:
		text = v
	case map[string]any:
		if val, ok := v["value"].(string); ok {
			text = val
		}
	}
	trimmed := strings.TrimSpace(text)
	if len([]rune(trimmed)) > 100000 || containsDisallowedControlChars(trimmed) {
		return ""
	}
	return trimmed
}

func validateSubjects(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return []string{}
	}
	var subjects []string
	seen := make(map[string]struct{})
	for _, item := range list {
		str, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(str)
		if trimmed == "" || len([]rune(trimmed)) > 100 || containsDisallowedControlChars(trimmed) {
			continue
		}
		if _, exists := seen[trimmed]; !exists {
			seen[trimmed] = struct{}{}
			subjects = append(subjects, trimmed)
		}
	}
	if subjects == nil {
		subjects = []string{}
	}
	return subjects
}

func validateLanguage(raw string) string {
	clean := CleanKey(raw)
	trimmed := strings.TrimSpace(clean)
	if err := domain.ValidateLanguageTag(trimmed); err != nil {
		return ""
	}
	return trimmed
}

func formatCoverURL(coverID any) *string {
	if coverID == nil {
		return nil
	}
	var num int64
	switch v := coverID.(type) {
	case float64:
		num = int64(v)
	case int:
		num = int64(v)
	case int64:
		num = v
	case json.Number:
		if n, err := v.Int64(); err == nil {
			num = n
		}
	case string:
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			num = n
		}
	}
	if num <= 0 {
		return nil
	}
	url := fmt.Sprintf("/api/v1/discover/covers/%d", num)
	return &url
}

// NormaliseSearchResponse decodes and normalises Open Library search JSON.
func NormaliseSearchResponse(body []byte, limit, offset int) (*NormalisedSearchResponse, error) {
	var raw struct {
		NumFound int              `json:"num_found"`
		Docs     []map[string]any `json:"docs"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "failed to parse Open Library search response"}
	}

	seenKeys := make(map[string]struct{})
	var items []NormalisedSearchResult

	for _, doc := range raw.Docs {
		keyRaw, _ := doc["key"].(string)
		workKey := CleanKey(keyRaw)
		if !IsValidWorkKey(workKey) {
			continue
		}
		// De-duplicate work keys
		if _, seen := seenKeys[workKey]; seen {
			continue
		}
		seenKeys[workKey] = struct{}{}

		titleRaw, _ := doc["title"].(string)
		title, ok := validateTitle(titleRaw)
		if !ok {
			// FR-5: list item missing valid title is dropped
			continue
		}

		// Zip-match author names and keys
		var authors []NormalisedAuthor
		var authorNames []string
		var authorKeys []string

		if names, ok := doc["author_name"].([]any); ok {
			for _, n := range names {
				if s, ok := n.(string); ok {
					authorNames = append(authorNames, s)
				}
			}
		}
		if keys, ok := doc["author_key"].([]any); ok {
			for _, k := range keys {
				if s, ok := k.(string); ok {
					authorKeys = append(authorKeys, s)
				}
			}
		}

		maxAuthors := len(authorNames)
		if len(authorKeys) > maxAuthors {
			maxAuthors = len(authorKeys)
		}

		for i := 0; i < maxAuthors; i++ {
			var name string
			if i < len(authorNames) {
				name = strings.TrimSpace(authorNames[i])
			}
			if name == "" || containsDisallowedControlChars(name) {
				continue
			}
			var keyPtr *string
			if i < len(authorKeys) {
				k := CleanKey(authorKeys[i])
				if openLibraryAuthorIDRegex.MatchString(k) {
					keyPtr = &k
				}
			}
			authors = append(authors, NormalisedAuthor{
				OpenLibraryAuthorKey: keyPtr,
				Name:                 name,
			})
		}
		if authors == nil {
			authors = []NormalisedAuthor{}
		}

		var firstPublishYear *int
		if yr, ok := doc["first_publish_year"].(float64); ok && yr > 0 {
			y := int(yr)
			firstPublishYear = &y
		}

		var editionCount int
		if ed, ok := doc["edition_count"].(float64); ok && ed >= 0 {
			editionCount = int(ed)
		}

		items = append(items, NormalisedSearchResult{
			OpenLibraryWorkKey: workKey,
			Title:              title,
			Authors:            authors,
			FirstPublishYear:   firstPublishYear,
			CoverURL:           formatCoverURL(doc["cover_i"]),
			EditionCount:       editionCount,
		})
	}

	if items == nil {
		items = []NormalisedSearchResult{}
	}

	return &NormalisedSearchResponse{
		Items:  items,
		Total:  raw.NumFound,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// NormaliseWork decodes and normalises an Open Library work JSON.
func NormaliseWork(body []byte) (*NormalisedWork, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "failed to parse Open Library work response"}
	}

	titleRaw, _ := raw["title"].(string)
	title, ok := validateTitle(titleRaw)
	if !ok {
		// FR-5: missing required title in top-level work returns Internal
		return nil, &domain.Error{Category: domain.Internal, Message: "upstream Open Library work record is missing a valid title"}
	}

	subtitleRaw, _ := raw["subtitle"].(string)
	subtitle := validateSubtitle(subtitleRaw)

	description := extractDescription(raw["description"])
	subjects := validateSubjects(raw["subjects"])

	var coverURL *string
	if covers, ok := raw["covers"].([]any); ok && len(covers) > 0 {
		coverURL = formatCoverURL(covers[0])
	}

	return &NormalisedWork{
		Title:       title,
		Subtitle:    subtitle,
		Description: description,
		Subjects:    subjects,
		Authors:     []NormalisedAuthor{}, // populated by client author fetches
		CoverURL:    coverURL,
	}, nil
}

// ExtractAuthorKeys extracts distinct Open Library author keys from a raw work payload.
func ExtractAuthorKeys(body []byte) []string {
	var raw struct {
		Authors []struct {
			Key    string `json:"key"`
			Author struct {
				Key string `json:"key"`
			} `json:"author"`
		} `json:"authors"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var keys []string
	for _, a := range raw.Authors {
		k := a.Author.Key
		if k == "" {
			k = a.Key
		}
		key := CleanKey(k)
		if !openLibraryAuthorIDRegex.MatchString(key) {
			continue
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}
	return keys
}

// NormaliseAuthor decodes and normalises an Open Library author JSON.
func NormaliseAuthor(key string, body []byte) (*NormalisedAuthor, error) {
	var raw struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "failed to parse author response"}
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" || containsDisallowedControlChars(name) {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "invalid author name"}
	}
	cleanKey := CleanKey(key)
	return &NormalisedAuthor{
		OpenLibraryAuthorKey: &cleanKey,
		Name:                 name,
	}, nil
}

// NormaliseEditions decodes and normalises an Open Library editions list JSON.
func NormaliseEditions(body []byte) ([]NormalisedEdition, error) {
	var raw struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, &domain.Error{Category: domain.Unavailable, Message: "failed to parse Open Library editions response"}
	}

	var editions []NormalisedEdition
	for _, entry := range raw.Entries {
		if len(editions) >= 50 {
			break
		}
		keyRaw, _ := entry["key"].(string)
		editionKey := CleanKey(keyRaw)
		if !openLibraryEditionIDRegex.MatchString(editionKey) {
			continue
		}

		titleRaw, _ := entry["title"].(string)
		title, ok := validateTitle(titleRaw)
		if !ok {
			// Omit edition without valid title
			continue
		}

		var publisher string
		if pubs, ok := entry["publishers"].([]any); ok && len(pubs) > 0 {
			if p, ok := pubs[0].(string); ok {
				p = strings.TrimSpace(p)
				if !containsDisallowedControlChars(p) && len([]rune(p)) <= 200 {
					publisher = p
				}
			}
		}

		var publishDate string
		if pd, ok := entry["publish_date"].(string); ok {
			pd = strings.TrimSpace(pd)
			if !containsDisallowedControlChars(pd) && len([]rune(pd)) <= 100 {
				publishDate = pd
			}
		}

		var language string
		if langs, ok := entry["languages"].([]any); ok && len(langs) > 0 {
			if lmap, ok := langs[0].(map[string]any); ok {
				if lkey, ok := lmap["key"].(string); ok {
					language = validateLanguage(lkey)
				}
			}
		}

		var coverURL *string
		if covers, ok := entry["covers"].([]any); ok && len(covers) > 0 {
			coverURL = formatCoverURL(covers[0])
		}

		editions = append(editions, NormalisedEdition{
			Title:                 title,
			Publisher:             publisher,
			PublishDate:           publishDate,
			Language:              language,
			OpenLibraryEditionKey: editionKey,
			CoverURL:              coverURL,
		})
	}

	if editions == nil {
		editions = []NormalisedEdition{}
	}

	return editions, nil
}
