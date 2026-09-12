package domain

import "context"

// IsInLibrary computes whether workID is "in the library": at least one
// of its Editions has a LibraryEntry, computed over the merge-resolved Edition set
// — the union of Editions across workID's canonical Work and everything merged
// into it, transitively.
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
