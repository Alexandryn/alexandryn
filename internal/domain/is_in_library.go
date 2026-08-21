package domain

import "context"

// IsInLibrary computes whether workID is "in the library": at least one
// of its Editions has a LibraryEntry (domain-library.md FR-2), computed
// over the **merge-resolved** Edition set — the union of Editions across
// workID's canonical Work and everything merged into it, transitively
// (FR-2's 2026-08-20 amendment, review 0048 finding 3's fix). Computed
// over raw parentage instead, this answers false for a book the user
// demonstrably owns the moment a match merges its Work.
func IsInLibrary(ctx context.Context, works WorkRepository, editions EditionRepository, entries LibraryEntryRepository, workID WorkID) (bool, error) {
	canonical, err := resolveCanonicalWork(ctx, works, workID)
	if err != nil {
		return false, err
	}
	group, err := collectMergeGroup(ctx, works, canonical)
	if err != nil {
		return false, err
	}

	for _, w := range group {
		editionsForWork, err := editions.FindByWork(ctx, w.ID())
		if err != nil {
			return false, err
		}
		for _, e := range editionsForWork {
			_, err := entries.FindByEdition(ctx, e.ID())
			if err == nil {
				return true, nil
			}
			if CategoryOf(err) != NotFound {
				return false, err
			}
		}
	}
	return false, nil
}
