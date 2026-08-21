package domain

// Bookmark (domain-reading.md FR-3/FR-4) MUST attach to Edition, not
// Work — EditionID is a required positional argument. Position is
// opaque (same "phase 11 decides the format" status as
// PrecisePosition.Value); Label is optional.
type Bookmark struct {
	id        BookmarkID
	editionID EditionID
	position  string
	label     string
}

func NewBookmark(id BookmarkID, editionID EditionID, position, label string) *Bookmark {
	return &Bookmark{id: id, editionID: editionID, position: position, label: label}
}

func (b *Bookmark) ID() BookmarkID { return b.id }

func (b *Bookmark) EditionID() EditionID { return b.editionID }

func (b *Bookmark) Position() string { return b.position }

func (b *Bookmark) Label() string { return b.label }

// Highlight (FR-3/FR-4) MUST attach to Edition and records a start and
// end position, both Edition-scoped. Note and category are optional.
//
// Not enforced here: FR-4's "end position not preceding start position."
// ADR 0020 carves this out as a single-value, construction-time
// invariant in principle — but Position's own format is deliberately
// undecided (domain-reading.md's Non-goals, same status as
// PrecisePosition.Value), and a generic ordering check over an opaque
// string (naive lexicographic comparison) would not mean "earlier in the
// book" for most real position formats a future CFI or page-number
// representation might use — it would be actively wrong, not just
// incomplete. Implementing it honestly needs the concrete format phase
// 11 picks; recorded here as a known gap, not silently skipped.
type Highlight struct {
	id            HighlightID
	editionID     EditionID
	startPosition string
	endPosition   string
	note          string
	category      string
}

func NewHighlight(id HighlightID, editionID EditionID, startPosition, endPosition, note, category string) *Highlight {
	return &Highlight{
		id:            id,
		editionID:     editionID,
		startPosition: startPosition,
		endPosition:   endPosition,
		note:          note,
		category:      category,
	}
}

func (h *Highlight) ID() HighlightID { return h.id }

func (h *Highlight) EditionID() EditionID { return h.editionID }

func (h *Highlight) StartPosition() string { return h.startPosition }

func (h *Highlight) EndPosition() string { return h.endPosition }

func (h *Highlight) Note() string { return h.note }

func (h *Highlight) Category() string { return h.category }
