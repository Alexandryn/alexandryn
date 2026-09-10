package openlibrary_test

import (
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
)

func TestNormaliseSearchDocs(t *testing.T) {
	t.Run("valid doc normalises correctly", func(t *testing.T) {
		raw := `
		{
			"num_found": 1,
			"docs": [
				{
					"key": "/works/OL82563W",
					"title": "Middlemarch",
					"author_key": ["OL21594A"],
					"author_name": ["George Eliot"],
					"first_publish_year": 1871,
					"cover_i": 8256301,
					"edition_count": 42
				}
			]
		}`

		resp, err := openlibrary.NormaliseSearchResponse([]byte(raw), 20, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Total != 1 || len(resp.Items) != 1 {
			t.Fatalf("expected 1 item, total 1; got total=%d len=%d", resp.Total, len(resp.Items))
		}
		item := resp.Items[0]
		if item.OpenLibraryWorkKey != "OL82563W" {
			t.Errorf("expected OL82563W, got %s", item.OpenLibraryWorkKey)
		}
		if item.Title != "Middlemarch" {
			t.Errorf("expected Middlemarch, got %s", item.Title)
		}
		if len(item.Authors) != 1 || item.Authors[0].Name != "George Eliot" || *item.Authors[0].OpenLibraryAuthorKey != "OL21594A" {
			t.Errorf("unexpected authors: %+v", item.Authors)
		}
		if item.FirstPublishYear == nil || *item.FirstPublishYear != 1871 {
			t.Errorf("expected 1871, got %v", item.FirstPublishYear)
		}
		if item.CoverURL == nil || *item.CoverURL != "/api/v1/discover/covers/8256301" {
			t.Errorf("expected /api/v1/discover/covers/8256301, got %v", item.CoverURL)
		}
		if item.EditionCount != 42 {
			t.Errorf("expected 42, got %d", item.EditionCount)
		}
	})

	t.Run("mismatched author array lengths handles null key gracefully", func(t *testing.T) {
		raw := `
		{
			"num_found": 1,
			"docs": [
				{
					"key": "/works/OL82563W",
					"title": "Collaborative Book",
					"author_key": ["OL111A"],
					"author_name": ["Author One", "Author Two"]
				}
			]
		}`
		resp, err := openlibrary.NormaliseSearchResponse([]byte(raw), 20, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(resp.Items))
		}
		authors := resp.Items[0].Authors
		if len(authors) != 2 {
			t.Fatalf("expected 2 authors, got %d", len(authors))
		}
		if authors[0].Name != "Author One" || *authors[0].OpenLibraryAuthorKey != "OL111A" {
			t.Errorf("unexpected author 0: %+v", authors[0])
		}
		if authors[1].Name != "Author Two" || authors[1].OpenLibraryAuthorKey != nil {
			t.Errorf("unexpected author 1: %+v (expected null key)", authors[1])
		}
	})

	t.Run("duplicate work keys in docs array are deduplicated", func(t *testing.T) {
		raw := `
		{
			"num_found": 2,
			"docs": [
				{
					"key": "/works/OL82563W",
					"title": "Middlemarch 1"
				},
				{
					"key": "/works/OL82563W",
					"title": "Middlemarch 2"
				}
			]
		}`
		resp, err := openlibrary.NormaliseSearchResponse([]byte(raw), 20, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Errorf("expected 1 item after deduplication, got %d", len(resp.Items))
		}
		if resp.Items[0].Title != "Middlemarch 1" {
			t.Errorf("expected first occurrence, got %s", resp.Items[0].Title)
		}
	})

	t.Run("doc missing required title is omitted", func(t *testing.T) {
		raw := `
		{
			"num_found": 2,
			"docs": [
				{
					"key": "/works/OL111W"
				},
				{
					"key": "/works/OL222W",
					"title": "Valid Book"
				}
			]
		}`
		resp, err := openlibrary.NormaliseSearchResponse([]byte(raw), 20, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(resp.Items))
		}
		if resp.Items[0].OpenLibraryWorkKey != "OL222W" {
			t.Errorf("expected OL222W, got %s", resp.Items[0].OpenLibraryWorkKey)
		}
	})

	t.Run("type-mismatched fields drop gracefully without failing unmarshal", func(t *testing.T) {
		raw := `
		{
			"num_found": 1,
			"docs": [
				{
					"key": "/works/OL82563W",
					"title": "Book With Wrong Types",
					"cover_i": "not-a-number",
					"first_publish_year": "nineteen-eighty",
					"edition_count": "many"
				}
			]
		}`
		resp, err := openlibrary.NormaliseSearchResponse([]byte(raw), 20, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(resp.Items))
		}
		item := resp.Items[0]
		if item.CoverURL != nil {
			t.Errorf("expected nil CoverURL for string cover_i, got %v", item.CoverURL)
		}
		if item.FirstPublishYear != nil {
			t.Errorf("expected nil FirstPublishYear for string year, got %v", item.FirstPublishYear)
		}
		if item.EditionCount != 0 {
			t.Errorf("expected default 0 for string edition_count, got %d", item.EditionCount)
		}
	})
}

func TestNormaliseWorkAndEditions(t *testing.T) {
	t.Run("missing work title returns domain.Internal", func(t *testing.T) {
		rawWork := `{"key": "/works/OL82563W"}`
		_, err := openlibrary.NormaliseWork([]byte(rawWork))
		if err == nil {
			t.Fatal("expected error for missing title, got nil")
		}
		domErr, ok := err.(*domain.Error)
		if !ok || domErr.Category != domain.Internal {
			t.Errorf("expected domain.Internal error, got %v", err)
		}
	})

	t.Run("control characters in description are dropped", func(t *testing.T) {
		rawWork := `{
			"key": "/works/OL82563W",
			"title": "Clean Title",
			"description": "Bad text with control char \u0007 bell"
		}`
		work, err := openlibrary.NormaliseWork([]byte(rawWork))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if work.Description != "" {
			t.Errorf("expected dropped description, got %q", work.Description)
		}
	})

	t.Run("normalises work description when formatted as string or object", func(t *testing.T) {
		rawObject := `{
			"key": "/works/OL82563W",
			"title": "Clean Title",
			"description": {"type": "/type/text", "value": "A great novel."}
		}`
		work, err := openlibrary.NormaliseWork([]byte(rawObject))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if work.Description != "A great novel." {
			t.Errorf("expected description 'A great novel.', got %q", work.Description)
		}
	})

	t.Run("normalises editions list and caps at 50", func(t *testing.T) {
		editionsJSON := `
		{
			"entries": [
				{
					"key": "/books/OL1M",
					"title": "Edition 1",
					"publishers": ["Publisher A"],
					"publish_date": "2020",
					"languages": [{"key": "/languages/eng"}],
					"covers": [1001]
				}
			]
		}`
		editions, err := openlibrary.NormaliseEditions([]byte(editionsJSON))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(editions) != 1 {
			t.Fatalf("expected 1 edition, got %d", len(editions))
		}
		ed := editions[0]
		if ed.OpenLibraryEditionKey != "OL1M" {
			t.Errorf("expected OL1M, got %s", ed.OpenLibraryEditionKey)
		}
		if ed.Publisher != "Publisher A" {
			t.Errorf("expected Publisher A, got %s", ed.Publisher)
		}
		if ed.Language != "eng" {
			t.Errorf("expected eng, got %s", ed.Language)
		}
		if ed.CoverURL == nil || *ed.CoverURL != "/api/v1/discover/covers/1001" {
			t.Errorf("expected /api/v1/discover/covers/1001, got %v", ed.CoverURL)
		}
	})

	t.Run("drops invalid BCP-47 language tag to empty string", func(t *testing.T) {
		editionsJSON := `
		{
			"entries": [
				{
					"key": "/books/OL1M",
					"title": "Edition With Bad Language",
					"languages": [{"key": "/languages/Invalid Language Tag!!!"}]
				}
			]
		}`
		editions, err := openlibrary.NormaliseEditions([]byte(editionsJSON))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(editions) != 1 {
			t.Fatalf("expected 1 edition, got %d", len(editions))
		}
		if editions[0].Language != "" {
			t.Errorf("expected empty language tag for invalid input, got %q", editions[0].Language)
		}
	})
}

func TestExtractAuthorKeys(t *testing.T) {
	raw := `{
		"authors": [
			{"author": {"key": "/authors/OL1A"}},
			{"key": "/authors/OL2A"},
			{"author": {"key": "/authors/OL1A"}},
			{"key": "invalid-key"}
		]
	}`

	keys := openlibrary.ExtractAuthorKeys([]byte(raw))
	if len(keys) != 2 {
		t.Fatalf("expected 2 distinct valid author keys, got %d (%v)", len(keys), keys)
	}
	if keys[0] != "OL1A" || keys[1] != "OL2A" {
		t.Errorf("expected [OL1A, OL2A], got %v", keys)
	}
}
