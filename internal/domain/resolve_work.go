package domain

import "context"

// ResolvedWork is the read-time transitive union: a
// canonical Work's authors, subjects, external references, and
// containment references, combined across that Work and every Work
// merged into it, transitively. Edition resolution is not this type's
// concern — Editions reference a Work by ID rather than being embedded
// fields on Work, so resolving "every Edition across a merge group" is a
// repository-level query IsInLibrary performs directly, using CanonicalID
// from here as its input.
type ResolvedWork struct {
	CanonicalID        WorkID
	Title              string
	Subtitle           string
	Authors            []AuthorID
	Subjects           []Subject
	OriginalLanguage   *Language
	ExternalReferences []ExternalReference
	Contains           []WorkID
}

// ResolveWork resolves id to its canonical Work and returns the union of
// authors/subjects/external references/containment references across
// that Work and everything merged into it, transitively. Title, Subtitle,
// and OriginalLanguage come from the canonical Work itself only — the
// canonical row's own scalar fields are authoritative, only collection
// fields union.
func ResolveWork(ctx context.Context, works WorkRepository, id WorkID) (*ResolvedWork, error) {
	canonical, err := resolveCanonicalWork(ctx, works, id)
	if err != nil {
		return nil, err
	}

	canonicalWork, err := works.FindByID(ctx, canonical)
	if err != nil {
		return nil, err
	}

	group, err := collectMergeGroup(ctx, works, canonical)
	if err != nil {
		return nil, err
	}

	result := &ResolvedWork{
		CanonicalID:      canonical,
		Title:            canonicalWork.Title(),
		Subtitle:         canonicalWork.Subtitle(),
		OriginalLanguage: canonicalWork.OriginalLanguage(),
	}

	seenAuthors := map[AuthorID]bool{}
	seenSubjects := map[Subject]bool{}
	seenRefs := map[ExternalReference]bool{}
	seenContains := map[WorkID]bool{}

	for _, w := range group {
		for _, a := range w.Authors() {
			if !seenAuthors[a] {
				seenAuthors[a] = true
				result.Authors = append(result.Authors, a)
			}
		}
		for _, s := range w.Subjects() {
			if !seenSubjects[s] {
				seenSubjects[s] = true
				result.Subjects = append(result.Subjects, s)
			}
		}
		for _, r := range w.ExternalReferences() {
			if !seenRefs[r] {
				seenRefs[r] = true
				result.ExternalReferences = append(result.ExternalReferences, r)
			}
		}
		for _, c := range w.Contains() {
			if !seenContains[c] {
				seenContains[c] = true
				result.Contains = append(result.Contains, c)
			}
		}
	}
	return result, nil
}

// resolveCanonicalWork follows MergedInto references starting at id
// until reaching a Work with no MergedInto, returning that canonical
// WorkID. Shared by ResolveWork, WorkMergeService, and
// WorkContainmentService — every place that needs "which Work does this
// ID actually mean, right now."
func resolveCanonicalWork(ctx context.Context, works WorkRepository, id WorkID) (WorkID, error) {
	current := id
	seen := map[WorkID]bool{}
	for {
		if seen[current] {
			return "", &Error{Category: Internal, Message: "merge chain contains an existing cycle"}
		}
		seen[current] = true

		w, err := works.FindByID(ctx, current)
		if err != nil {
			return "", err
		}
		if w.MergedInto() == nil {
			return current, nil
		}
		current = *w.MergedInto()
	}
}

// collectMergeGroup returns canonical's own Work plus every Work merged
// into it, transitively (repeated FindMergedInto hops).
func collectMergeGroup(ctx context.Context, works WorkRepository, canonical WorkID) ([]*Work, error) {
	canonicalWork, err := works.FindByID(ctx, canonical)
	if err != nil {
		return nil, err
	}
	group := []*Work{canonicalWork}

	queue := []WorkID{canonical}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		members, err := works.FindMergedInto(ctx, current)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			group = append(group, m)
			queue = append(queue, m.ID())
		}
	}
	return group, nil
}

// containsClosure returns the transitive, merge-resolved set of WorkIDs
// that start's canonical Work contains: every Contains edge across
// start's own merge group, with each contained reference itself resolved
// to canonical form before continuing the walk. Shared by
// WorkContainmentService (checking a new Contains edge) and
// WorkMergeService (checking whether a merge would, once resolved, make
// a Work's own contains-closure reach itself).
func containsClosure(ctx context.Context, works WorkRepository, start WorkID) (map[WorkID]bool, error) {
	startCanonical, err := resolveCanonicalWork(ctx, works, start)
	if err != nil {
		return nil, err
	}

	closure := map[WorkID]bool{}
	queue := []WorkID{startCanonical}
	visitedGroups := map[WorkID]bool{}

	for len(queue) > 0 {
		canonical := queue[0]
		queue = queue[1:]
		if visitedGroups[canonical] {
			continue
		}
		visitedGroups[canonical] = true

		group, err := collectMergeGroup(ctx, works, canonical)
		if err != nil {
			return nil, err
		}
		for _, w := range group {
			for _, containedID := range w.Contains() {
				resolved, err := resolveCanonicalWork(ctx, works, containedID)
				if err != nil {
					return nil, err
				}
				if !closure[resolved] {
					closure[resolved] = true
					queue = append(queue, resolved)
				}
			}
		}
	}
	return closure, nil
}
