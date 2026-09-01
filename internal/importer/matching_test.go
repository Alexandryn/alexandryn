package importer_test

import (
	"context"
	"testing"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

type fakeLibraryFinder struct {
	hits []importer.ExistingEditionHit
}

func (f *fakeLibraryFinder) FindOwnedByISBN(ctx context.Context, isbn string) ([]importer.ExistingEditionHit, error) {
	return f.hits, nil
}

type fakeOpenLibrarySearcher struct {
	results []openlibrary.NormalisedSearchResult
	err     error
}

func (f *fakeOpenLibrarySearcher) Search(ctx context.Context, q string, limit, offset int) (*openlibrary.NormalisedSearchResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &openlibrary.NormalisedSearchResponse{
		Items: f.results,
		Total: len(f.results),
	}, nil
}

func TestMatching_ExistingLibraryExactHit(t *testing.T) {
	ctx := context.Background()
	isbn := "9780441172719"
	meta, _ := extract.NewExtractedMetadata("Dune", []string{"Frank Herbert"}, &isbn, nil, nil, nil, nil, extract.FormatEPUB)

	libFinder := &fakeLibraryFinder{
		hits: []importer.ExistingEditionHit{
			{
				EditionID: "ed-dune-1",
				WorkID:    "work-dune-1",
				Title:     "Dune",
				Author:    "Frank Herbert",
				ISBN:      "9780441172719",
			},
		},
	}
	olSearcher := &fakeOpenLibrarySearcher{}

	matcher := importer.NewMatcher(libFinder, olSearcher)
	matches, autoAccept, err := matcher.Match(ctx, meta)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	if !autoAccept {
		t.Error("expected autoAccept to be true for exactly one exact ISBN match")
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}
	if matches[0].Type != importer.MatchCandidateTypeExistingEdition || matches[0].Confidence != importer.MatchConfidenceExact {
		t.Errorf("match = %+v, want exact existing_edition", matches[0])
	}
	if matches[0].EditionID == nil || *matches[0].EditionID != "ed-dune-1" {
		t.Errorf("editionId = %v, want ed-dune-1", matches[0].EditionID)
	}
}

func TestMatching_MultipleExistingLibraryHitsDoesNotAutoAccept(t *testing.T) {
	ctx := context.Background()
	isbn := "9780441172719"
	meta, _ := extract.NewExtractedMetadata("Dune", []string{"Frank Herbert"}, &isbn, nil, nil, nil, nil, extract.FormatEPUB)

	libFinder := &fakeLibraryFinder{
		hits: []importer.ExistingEditionHit{
			{EditionID: "ed-dune-1", Title: "Dune", ISBN: isbn},
			{EditionID: "ed-dune-2", Title: "Dune Special Edition", ISBN: isbn},
		},
	}
	olSearcher := &fakeOpenLibrarySearcher{}

	matcher := importer.NewMatcher(libFinder, olSearcher)
	matches, autoAccept, err := matcher.Match(ctx, meta)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	if autoAccept {
		t.Error("autoAccept must be false when multiple existing library editions match")
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want 2", len(matches))
	}
}

func TestMatching_OpenLibraryHighConfidenceDoesNotAutoAccept(t *testing.T) {
	ctx := context.Background()
	meta, _ := extract.NewExtractedMetadata("Neuromancer", []string{"William Gibson"}, nil, nil, nil, nil, nil, extract.FormatEPUB)

	authorName := "William Gibson"
	authorKey := "OL123A"
	olSearcher := &fakeOpenLibrarySearcher{
		results: []openlibrary.NormalisedSearchResult{
			{
				OpenLibraryWorkKey: "OL456W",
				Title:              "Neuromancer",
				Authors: []openlibrary.NormalisedAuthor{
					{Name: authorName, OpenLibraryAuthorKey: &authorKey},
				},
			},
		},
	}
	libFinder := &fakeLibraryFinder{}

	matcher := importer.NewMatcher(libFinder, olSearcher)
	matches, autoAccept, err := matcher.Match(ctx, meta)
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	if autoAccept {
		t.Fatal("autoAccept MUST be false for Open Library matches without existing library match")
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1", len(matches))
	}
	if matches[0].Confidence != importer.MatchConfidenceHigh {
		t.Errorf("confidence = %v, want high", matches[0].Confidence)
	}
}

func TestMatching_ConfusableInputShapesCappedAtMedium(t *testing.T) {
	matcher := importer.NewMatcher(&fakeLibraryFinder{}, nil)

	// Shape 1: Parenthetical qualifier difference ("Series" vs "Series (Color)")
	meta1, _ := extract.NewExtractedMetadata("Scott Pilgrim", []string{"Bryan Lee O'Malley"}, nil, nil, nil, nil, nil, extract.FormatCBZ)
	res1 := openlibrary.NormalisedSearchResult{
		Title: "Scott Pilgrim (Color)",
		Authors: []openlibrary.NormalisedAuthor{
			{Name: "Bryan Lee O'Malley"},
		},
	}
	conf1 := matcher.ScoreOpenLibraryResult(meta1, res1)
	if conf1 != importer.MatchConfidenceMedium && conf1 != importer.MatchConfidenceLow {
		t.Errorf("parenthetical qualifier score = %v, want <= medium", conf1)
	}

	// Shape 2: Same-year / series volume collision
	meta2, _ := extract.NewExtractedMetadata("Saga Vol. 1", []string{"Brian K. Vaughan"}, nil, nil, nil, nil, nil, extract.FormatCBZ)
	res2 := openlibrary.NormalisedSearchResult{
		Title: "Saga Vol. 2",
		Authors: []openlibrary.NormalisedAuthor{
			{Name: "Brian K. Vaughan"},
		},
	}
	conf2 := matcher.ScoreOpenLibraryResult(meta2, res2)
	if conf2 != importer.MatchConfidenceLow {
		t.Errorf("different volume score = %v, want low", conf2)
	}

	// Shape 3: Standalone title colliding with unrelated series name
	meta3, _ := extract.NewExtractedMetadata("Foundation", []string{"John Author"}, nil, nil, nil, nil, nil, extract.FormatPDF)
	res3 := openlibrary.NormalisedSearchResult{
		Title: "Foundation",
		Authors: []openlibrary.NormalisedAuthor{
			{Name: "Isaac Asimov"},
		},
	}
	conf3 := matcher.ScoreOpenLibraryResult(meta3, res3)
	if conf3 != importer.MatchConfidenceMedium {
		t.Errorf("title match with different author score = %v, want medium", conf3)
	}
}
