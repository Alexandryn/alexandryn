package content

import (
	"archive/zip"
	"context"
	"errors"
	"io"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
)

// OwnershipRepo confirms an Edition is actually owned before any byte is
// fetched (backend-reader-content.md FR-1) — FindByEdition returns a
// *domain.Error with category NotFound when it is not.
type OwnershipRepo interface {
	FindByEdition(ctx context.Context, editionID domain.EditionID) (*domain.LibraryEntry, error)
}

// OfferingRepo lists the SourceOfferings for an Edition, most recently
// observed first (FR-2's fallback order).
type OfferingRepo interface {
	FindByEdition(ctx context.Context, editionID domain.EditionID) ([]*domain.SourceOffering, error)
}

// SourceResolver opens the bytes behind a FileReference against a
// registered source — the same capability phase 10's import path uses
// (backend-source-adapter.md Provider.Resolve, wrapped).
type SourceResolver interface {
	Resolve(ctx context.Context, sourceID string, ref domain.FileReference) (io.ReadCloser, error)
}

// Resolver is the Cache's Loader: it checks ownership, resolves the
// Edition's bytes through each SourceOffering in turn, materialises the
// stream through phase 10's spool (reusing its 250 MiB cap), and opens a
// zip.Reader — applying phase 10's entry-count cap once here (FR-4).
type Resolver struct {
	ownership OwnershipRepo
	offerings OfferingRepo
	sources   SourceResolver
}

func NewResolver(ownership OwnershipRepo, offerings OfferingRepo, sources SourceResolver) *Resolver {
	return &Resolver{ownership: ownership, offerings: offerings, sources: sources}
}

// Load implements Loader.
func (r *Resolver) Load(ctx context.Context, editionID domain.EditionID) (*zip.Reader, func(), error) {
	if _, err := r.ownership.FindByEdition(ctx, editionID); err != nil {
		return nil, nil, err // NotFound for an unowned Edition propagates unchanged
	}

	offs, err := r.offerings.FindByEdition(ctx, editionID)
	if err != nil {
		return nil, nil, err
	}
	if len(offs) == 0 {
		return nil, nil, &domain.Error{Category: domain.Unavailable, Message: "no source offering for this edition"}
	}

	var stream io.ReadCloser
	for _, off := range offs {
		s, rerr := r.sources.Resolve(ctx, string(off.SourceID()), off.FileReference())
		if rerr == nil && s != nil {
			stream = s
			break
		}
	}
	if stream == nil {
		return nil, nil, &domain.Error{Category: domain.Unavailable, Message: "this book's source isn't reachable right now"}
	}
	defer stream.Close()

	tmp, cleanup, err := extract.Materialize(ctx, stream)
	if err != nil {
		return nil, nil, materializeError(err)
	}

	info, err := tmp.Stat()
	if err != nil {
		cleanup()
		return nil, nil, &domain.Error{Category: domain.Unavailable, Message: "could not read the materialised book file"}
	}

	zr, err := zip.NewReader(tmp, info.Size())
	if err != nil {
		cleanup()
		return nil, nil, &domain.Error{Category: domain.Unavailable, Message: "this book's file is not a readable EPUB archive"}
	}
	if len(zr.File) > extract.MaxZipEntryCount {
		cleanup()
		return nil, nil, &domain.Error{Category: domain.Unavailable, Message: "this book's file has too many entries to open safely"}
	}

	return zr, cleanup, nil
}

func materializeError(err error) error {
	switch {
	case errors.Is(err, extract.ErrOversized):
		return &domain.Error{Category: domain.Unavailable, Message: "this book's file is too large to open"}
	case errors.Is(err, extract.ErrMalformed):
		return &domain.Error{Category: domain.Unavailable, Message: "this book's file could not be read"}
	case errors.Is(err, context.DeadlineExceeded):
		return &domain.Error{Category: domain.Unavailable, Message: "reading this book's file timed out"}
	default:
		var derr *domain.Error
		if errors.As(err, &derr) {
			return derr
		}
		return &domain.Error{Category: domain.Unavailable, Message: "this book's source isn't reachable right now"}
	}
}
