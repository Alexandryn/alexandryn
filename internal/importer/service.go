package importer

import (
	"context"
	"fmt"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/openlibrary"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

// CandidateRepository is the repository interface for import candidates.
type CandidateRepository interface {
	Create(ctx context.Context, rec postgres.ImportCandidateRecord) error
	Get(ctx context.Context, id string) (postgres.ImportCandidateRecord, error)
	List(ctx context.Context, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error)
	UpdateStatus(ctx context.Context, id string, status string, now time.Time) error
	UpdateExtractedAndMatches(ctx context.Context, id string, status string, extracted []byte, matches []byte, now time.Time) error
	UpdateFailed(ctx context.Context, id string, lastError string, now time.Time) error
	ExistsBySourceAndFileRefID(ctx context.Context, sourceID string, fileRefID string) (bool, error)
}

// Service manages import candidate lifecycle, auto-acceptance, and user confirmation (backend-import-pipeline.md FR-5, FR-6, FR-7).
type Service struct {
	works           domain.WorkRepository
	editions        domain.EditionRepository
	authors         domain.AuthorRepository
	sourceOfferings domain.SourceOfferingRepository
	libraryEntries  domain.LibraryEntryRepository
	candidates      CandidateRepository
	libraryService  *domain.LibraryService
	transactor      domain.Transactor
	ids             domain.IDGenerator
}

// NewService constructs an importer Service.
func NewService(
	works domain.WorkRepository,
	editions domain.EditionRepository,
	authors domain.AuthorRepository,
	sourceOfferings domain.SourceOfferingRepository,
	libraryEntries domain.LibraryEntryRepository,
	candidates CandidateRepository,
	libraryService *domain.LibraryService,
	transactor domain.Transactor,
	ids domain.IDGenerator,
) *Service {
	return &Service{
		works:           works,
		editions:        editions,
		authors:         authors,
		sourceOfferings: sourceOfferings,
		libraryEntries:  libraryEntries,
		candidates:      candidates,
		libraryService:  libraryService,
		transactor:      transactor,
		ids:             ids,
	}
}

// AutoImport attaches a verified file to an existing owned Edition in one transaction (FR-5, FR-6).
// It updates the persisted FileReference format to the content-sniffed verified format.
func (s *Service) AutoImport(
	ctx context.Context,
	candidateID string,
	sourceID string,
	editionID string,
	fileRef domain.FileReference,
	verifiedFormat extract.Format,
	now time.Time,
) error {
	vRef, err := domain.NewFileReference(fileRef.ReferenceID, string(verifiedFormat), fileRef.SizeBytes)
	if err != nil {
		return fmt.Errorf("building verified file reference: %w", err)
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		offeringID := domain.SourceOfferingID(s.ids.NewID())
		offering := domain.NewSourceOffering(offeringID, domain.SourceID(sourceID), domain.EditionID(editionID), vRef, now)
		if err := s.sourceOfferings.Save(txCtx, offering); err != nil {
			return err
		}

		if _, _, err := s.libraryService.AddEntry(txCtx, domain.EditionID(editionID), now); err != nil {
			return err
		}

		return s.candidates.UpdateStatus(txCtx, candidateID, postgres.ImportCandidateStatusAutoImported, now)
	})
}

// ConfirmAttachExisting attaches a pending candidate to a user-selected existing Edition (FR-7).
func (s *Service) ConfirmAttachExisting(ctx context.Context, candidateID string, editionID string, now time.Time) error {
	cand, err := s.candidates.Get(ctx, candidateID)
	if err != nil {
		return err
	}
	if cand.Status != postgres.ImportCandidateStatusPending {
		return &domain.Error{Category: domain.Conflict, Message: fmt.Sprintf("candidate %s is not in pending status", candidateID)}
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		offeringID := domain.SourceOfferingID(s.ids.NewID())
		offering := domain.NewSourceOffering(offeringID, domain.SourceID(cand.SourceID), domain.EditionID(editionID), cand.FileReference, now)
		if err := s.sourceOfferings.Save(txCtx, offering); err != nil {
			return err
		}

		if _, _, err := s.libraryService.AddEntry(txCtx, domain.EditionID(editionID), now); err != nil {
			return err
		}

		return s.candidates.UpdateStatus(txCtx, candidateID, postgres.ImportCandidateStatusConfirmed, now)
	})
}

// ConfirmCreateNew creates a new Work, Edition, and Author from extracted metadata alone (FR-7).
func (s *Service) ConfirmCreateNew(ctx context.Context, candidateID string, meta extract.ExtractedMetadata, now time.Time) error {
	cand, err := s.candidates.Get(ctx, candidateID)
	if err != nil {
		return err
	}
	if cand.Status != postgres.ImportCandidateStatusPending {
		return &domain.Error{Category: domain.Conflict, Message: fmt.Sprintf("candidate %s is not in pending status", candidateID)}
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var authorIDs []domain.AuthorID
		for _, aName := range meta.Authors {
			aID := domain.AuthorID(s.ids.NewID())
			author, err := domain.NewAuthor(aID, aName, nil)
			if err != nil {
				continue
			}
			if err := s.authors.Save(txCtx, author); err == nil {
				authorIDs = append(authorIDs, aID)
			}
		}

		workID := domain.WorkID(s.ids.NewID())
		work, err := domain.NewWork(workID, meta.Title, "", authorIDs, nil, nil, nil)
		if err != nil {
			return err
		}
		if err := s.works.Save(txCtx, work); err != nil {
			return err
		}

		editionID := domain.EditionID(s.ids.NewID())
		lang, _ := domain.NewLanguage("en")
		if meta.Language != nil && *meta.Language != "" {
			if l, err := domain.NewLanguage(*meta.Language); err == nil {
				lang = l
			}
		}
		pub := ""
		if meta.Publisher != nil {
			pub = *meta.Publisher
		}

		edition, err := domain.NewEdition(editionID, workID, lang, meta.ISBN, pub, nil, nil)
		if err != nil {
			return err
		}
		if err := s.editions.Save(txCtx, edition); err != nil {
			return err
		}

		offeringID := domain.SourceOfferingID(s.ids.NewID())
		offering := domain.NewSourceOffering(offeringID, domain.SourceID(cand.SourceID), editionID, cand.FileReference, now)
		if err := s.sourceOfferings.Save(txCtx, offering); err != nil {
			return err
		}

		if _, _, err := s.libraryService.AddEntry(txCtx, editionID, now); err != nil {
			return err
		}

		return s.candidates.UpdateStatus(txCtx, candidateID, postgres.ImportCandidateStatusConfirmed, now)
	})
}

// ConfirmOpenLibraryMatch constructs Work/Edition from Open Library detail response (FR-7).
func (s *Service) ConfirmOpenLibraryMatch(
	ctx context.Context,
	candidateID string,
	openLibraryWorkKey string,
	meta extract.ExtractedMetadata,
	olDetail *openlibrary.DiscoverWorkDetail,
	now time.Time,
) error {
	cand, err := s.candidates.Get(ctx, candidateID)
	if err != nil {
		return err
	}
	if cand.Status != postgres.ImportCandidateStatusPending {
		return &domain.Error{Category: domain.Conflict, Message: fmt.Sprintf("candidate %s is not in pending status", candidateID)}
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		var authorIDs []domain.AuthorID
		var authorNames []string
		if olDetail != nil && len(olDetail.Work.Authors) > 0 {
			for _, a := range olDetail.Work.Authors {
				authorNames = append(authorNames, a.Name)
			}
		} else {
			authorNames = meta.Authors
		}

		for _, aName := range authorNames {
			aID := domain.AuthorID(s.ids.NewID())
			author, err := domain.NewAuthor(aID, aName, nil)
			if err != nil {
				continue
			}
			if err := s.authors.Save(txCtx, author); err == nil {
				authorIDs = append(authorIDs, aID)
			}
		}

		workTitle := meta.Title
		workSubtitle := ""
		if olDetail != nil && olDetail.Work.Title != "" {
			workTitle = olDetail.Work.Title
			workSubtitle = olDetail.Work.Subtitle
		}

		var workExtRefs []domain.ExternalReference
		if openLibraryWorkKey != "" {
			workExtRefs = append(workExtRefs, domain.ExternalReference{Source: "openlibrary", ID: openLibraryWorkKey})
		}

		workID := domain.WorkID(s.ids.NewID())
		work, err := domain.NewWork(workID, workTitle, workSubtitle, authorIDs, nil, nil, workExtRefs)
		if err != nil {
			return err
		}
		if err := s.works.Save(txCtx, work); err != nil {
			return err
		}

		editionID := domain.EditionID(s.ids.NewID())
		lang, _ := domain.NewLanguage("en")
		pub := ""
		if meta.Language != nil && *meta.Language != "" {
			if l, err := domain.NewLanguage(*meta.Language); err == nil {
				lang = l
			}
		}
		if meta.Publisher != nil {
			pub = *meta.Publisher
		}

		if olDetail != nil && len(olDetail.Editions) > 0 {
			ed := olDetail.Editions[0]
			if ed.Language != "" {
				if l, err := domain.NewLanguage(ed.Language); err == nil {
					lang = l
				}
			}
			if ed.Publisher != "" {
				pub = ed.Publisher
			}
		}

		edition, err := domain.NewEdition(editionID, workID, lang, meta.ISBN, pub, nil, nil)
		if err != nil {
			return err
		}
		if err := s.editions.Save(txCtx, edition); err != nil {
			return err
		}

		offeringID := domain.SourceOfferingID(s.ids.NewID())
		offering := domain.NewSourceOffering(offeringID, domain.SourceID(cand.SourceID), editionID, cand.FileReference, now)
		if err := s.sourceOfferings.Save(txCtx, offering); err != nil {
			return err
		}

		if _, _, err := s.libraryService.AddEntry(txCtx, editionID, now); err != nil {
			return err
		}

		return s.candidates.UpdateStatus(txCtx, candidateID, postgres.ImportCandidateStatusConfirmed, now)
	})
}

// Reject transitions a pending candidate to rejected (FR-7).
func (s *Service) Reject(ctx context.Context, candidateID string, now time.Time) error {
	cand, err := s.candidates.Get(ctx, candidateID)
	if err != nil {
		return err
	}
	if cand.Status != postgres.ImportCandidateStatusPending {
		return &domain.Error{Category: domain.Conflict, Message: fmt.Sprintf("candidate %s is not in pending status", candidateID)}
	}

	return s.candidates.UpdateStatus(ctx, candidateID, postgres.ImportCandidateStatusRejected, now)
}
