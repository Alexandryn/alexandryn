package domain

import "time"

// Bookmark attaches to Edition, not Work — EditionID is a required
// positional argument. Position is an opaque edition-scoped position
// string; Label is optional. CreatedAt records when the mark was made.
type Bookmark struct {
	id        BookmarkID
	editionID EditionID
	position  string
	label     string
	createdAt time.Time
}

func NewBookmark(id BookmarkID, editionID EditionID, position, label string, createdAt time.Time) *Bookmark {
	return &Bookmark{id: id, editionID: editionID, position: position, label: label, createdAt: createdAt}
}

func (b *Bookmark) ID() BookmarkID { return b.id }

func (b *Bookmark) EditionID() EditionID { return b.editionID }

func (b *Bookmark) Position() string { return b.position }

func (b *Bookmark) Label() string { return b.label }

func (b *Bookmark) CreatedAt() time.Time { return b.createdAt }

// Highlight attaches to Edition and records a start and end position,
// both Edition-scoped. Note and category are optional.
type Highlight struct {
	id            HighlightID
	editionID     EditionID
	startPosition string
	endPosition   string
	note          string
	category      string
	createdAt     time.Time
}

func NewHighlight(id HighlightID, editionID EditionID, startPosition, endPosition, note, category string, createdAt time.Time) *Highlight {
	return &Highlight{
		id:            id,
		editionID:     editionID,
		startPosition: startPosition,
		endPosition:   endPosition,
		note:          note,
		category:      category,
		createdAt:     createdAt,
	}
}

func (h *Highlight) ID() HighlightID { return h.id }

func (h *Highlight) EditionID() EditionID { return h.editionID }

func (h *Highlight) StartPosition() string { return h.startPosition }

func (h *Highlight) EndPosition() string { return h.endPosition }

func (h *Highlight) Note() string { return h.note }

func (h *Highlight) Category() string { return h.category }

func (h *Highlight) CreatedAt() time.Time { return h.createdAt }
