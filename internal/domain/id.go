package domain

// ID types, one per aggregate (E0, tasks/plan-phase02-domain.md). Distinct
// named types rather than one bare string type: passing a WorkID where an
// AuthorID is expected fails to compile, at zero runtime cost — no spec
// mandates this shape, it's a deliberate choice recorded in the plan.
type (
	WorkID            string
	EditionID         string
	AuthorID          string
	LibraryEntryID    string
	CollectionID      string
	SourceID          string
	SourceOfferingID  string
	ReadingProgressID string
	BookmarkID        string
	HighlightID       string
)

// IDGenerator produces a new, unique identifier value at aggregate
// construction time (domain-bibliographic.md FR-1: "generated at creation,
// never derived from external data"). The single string-returning method
// is deliberately untyped per-aggregate — callers convert the result to
// the ID type they need (WorkID(gen.NewID())) — so one interface and one
// injected implementation serves every aggregate, matching this project's
// existing Clock/FS injection pattern (go-backend-conventions/SKILL.md).
//
// internal/testutil.FakeIDGenerator already implements this shape (built
// in phase 03's T1, before this interface existed) — E1 declares the
// interface to match the fake, not the other way around.
type IDGenerator interface {
	NewID() string
}
