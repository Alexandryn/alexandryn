//go:build integration

package importer_test

import (
	"context"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/idgen"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/importer/extract"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestService_AutoImport_NeverCreatesNewWorkOrEdition(t *testing.T) {
	ctx := context.Background()
	pool := schemaTestPool(t)

	workRepo := postgres.NewWorkRepository(pool)
	editionRepo := postgres.NewEditionRepository(pool)
	entryRepo := postgres.NewLibraryEntryRepository(pool)
	offeringRepo := postgres.NewSourceOfferingRepository(pool)
	candRepo := postgres.NewImportCandidateRepository(pool)
	authorRepo := postgres.NewAuthorRepository(pool)
	transactor := postgres.NewTransactor(pool)
	idGen := idgen.New()
	librarySvc := domain.NewLibraryService(editionRepo, entryRepo)

	svc := importer.NewService(
		workRepo,
		editionRepo,
		authorRepo,
		offeringRepo,
		entryRepo,
		candRepo,
		librarySvc,
		transactor,
		idGen,
	)

	// Seed source, work, edition, and initial library entry
	mustExecPool(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download)
		VALUES ('src-auto-1', 'Source 1', 'local-folder', true, true, true) ON CONFLICT DO NOTHING`)

	workID := domain.WorkID(idGen.NewID())
	work, err := domain.NewWork(workID, "Existing Book", "", nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("NewWork: %v", err)
	}
	if err := workRepo.Save(ctx, work); err != nil {
		t.Fatalf("Save work: %v", err)
	}

	editionID := domain.EditionID(idGen.NewID())
	isbn := "9780441172719"
	lang, _ := domain.NewLanguage("en")
	edition, err := domain.NewEdition(editionID, workID, lang, &isbn, "Chilton", nil, nil)
	if err != nil {
		t.Fatalf("NewEdition: %v", err)
	}
	if err := editionRepo.Save(ctx, edition); err != nil {
		t.Fatalf("Save edition: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if _, _, err := librarySvc.AddEntry(ctx, editionID, now); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}

	// Create import candidate row
	ref, _ := domain.NewFileReference("ref-file-1", "pdf", nil) // declared as pdf
	cand := postgres.ImportCandidateRecord{
		ID:            "cand-auto-1",
		SourceID:      "src-auto-1",
		FileReference: ref,
		Status:        postgres.ImportCandidateStatusQueued,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := candRepo.Create(ctx, cand); err != nil {
		t.Fatalf("Create cand: %v", err)
	}

	// Auto-import with verified format epub (FR-6: corrects format to content-sniffed epub)
	if err := svc.AutoImport(ctx, "cand-auto-1", "src-auto-1", string(editionID), ref, extract.FormatEPUB, now); err != nil {
		t.Fatalf("AutoImport: %v", err)
	}

	// 1. Verify candidate is status auto_imported
	updatedCand, err := candRepo.Get(ctx, "cand-auto-1")
	if err != nil {
		t.Fatalf("Get cand: %v", err)
	}
	if updatedCand.Status != postgres.ImportCandidateStatusAutoImported {
		t.Fatalf("status = %q, want auto_imported", updatedCand.Status)
	}

	// 2. Verify SourceOffering exists with verified format 'epub'
	offerings, err := offeringRepo.FindBySource(ctx, "src-auto-1")
	if err != nil {
		t.Fatalf("FindBySource: %v", err)
	}
	if len(offerings) != 1 {
		t.Fatalf("offerings count = %d, want 1", len(offerings))
	}
	if offerings[0].EditionID() != editionID {
		t.Fatalf("offering edition = %v, want %v", offerings[0].EditionID(), editionID)
	}
	if offerings[0].FileReference().Format != "epub" {
		t.Fatalf("offering format = %q, want epub", offerings[0].FileReference().Format)
	}

	// 3. Verify exactly one LibraryEntry exists
	entry, err := entryRepo.FindByEdition(ctx, editionID)
	if err != nil {
		t.Fatalf("FindByEdition: %v", err)
	}
	if entry == nil {
		t.Fatal("expected library entry to exist")
	}

	// 4. Verify NO new works were created (works count is still 1)
	works, err := workRepo.QueryLibrary(ctx, domain.LibraryQuery{Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary: %v", err)
	}
	if len(works.Works) != 1 {
		t.Fatalf("Total works = %d, want exactly 1", len(works.Works))
	}
}

func TestService_Confirm_CreateNew(t *testing.T) {
	ctx := context.Background()
	pool := schemaTestPool(t)

	workRepo := postgres.NewWorkRepository(pool)
	editionRepo := postgres.NewEditionRepository(pool)
	entryRepo := postgres.NewLibraryEntryRepository(pool)
	offeringRepo := postgres.NewSourceOfferingRepository(pool)
	candRepo := postgres.NewImportCandidateRepository(pool)
	authorRepo := postgres.NewAuthorRepository(pool)
	transactor := postgres.NewTransactor(pool)
	idGen := idgen.New()
	librarySvc := domain.NewLibraryService(editionRepo, entryRepo)

	svc := importer.NewService(
		workRepo,
		editionRepo,
		authorRepo,
		offeringRepo,
		entryRepo,
		candRepo,
		librarySvc,
		transactor,
		idGen,
	)

	mustExecPool(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download)
		VALUES ('src-new-1', 'Source New', 'local-folder', true, true, true) ON CONFLICT DO NOTHING`)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref, _ := domain.NewFileReference("ref-new-book", "epub", nil)
	cand := postgres.ImportCandidateRecord{
		ID:            "cand-new-1",
		SourceID:      "src-new-1",
		FileReference: ref,
		Status:        postgres.ImportCandidateStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := candRepo.Create(ctx, cand); err != nil {
		t.Fatalf("Create cand: %v", err)
	}

	pub := "Indie Press"
	meta, err := extract.NewExtractedMetadata("Brand New Book", []string{"New Author"}, nil, nil, &pub, nil, nil, extract.FormatEPUB)
	if err != nil {
		t.Fatalf("NewExtractedMetadata: %v", err)
	}

	if err := svc.ConfirmCreateNew(ctx, "cand-new-1", meta, now); err != nil {
		t.Fatalf("ConfirmCreateNew: %v", err)
	}

	// Verify candidate is confirmed
	c, err := candRepo.Get(ctx, "cand-new-1")
	if err != nil {
		t.Fatalf("Get cand: %v", err)
	}
	if c.Status != postgres.ImportCandidateStatusConfirmed {
		t.Fatalf("status = %q, want confirmed", c.Status)
	}

	// Verify work was created
	page, err := workRepo.QueryLibrary(ctx, domain.LibraryQuery{Limit: 10})
	if err != nil {
		t.Fatalf("QueryLibrary: %v", err)
	}
	var found bool
	for _, w := range page.Works {
		if w.Title == "Brand New Book" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected newly created work in library")
	}

	// Confirming again returns Conflict (409)
	err = svc.ConfirmCreateNew(ctx, "cand-new-1", meta, now)
	if err == nil || domain.CategoryOf(err) != domain.Conflict {
		t.Fatalf("second confirm err = %v, want Conflict", err)
	}
}

func TestService_Reject(t *testing.T) {
	ctx := context.Background()
	pool := schemaTestPool(t)

	candRepo := postgres.NewImportCandidateRepository(pool)
	entryRepo := postgres.NewLibraryEntryRepository(pool)
	editionRepo := postgres.NewEditionRepository(pool)
	workRepo := postgres.NewWorkRepository(pool)
	offeringRepo := postgres.NewSourceOfferingRepository(pool)
	authorRepo := postgres.NewAuthorRepository(pool)
	transactor := postgres.NewTransactor(pool)
	idGen := idgen.New()
	librarySvc := domain.NewLibraryService(editionRepo, entryRepo)

	svc := importer.NewService(
		workRepo,
		editionRepo,
		authorRepo,
		offeringRepo,
		entryRepo,
		candRepo,
		librarySvc,
		transactor,
		idGen,
	)

	mustExecPool(t, pool, `INSERT INTO sources (id, label, kind, can_list, can_search, can_download)
		VALUES ('src-rej-1', 'Source Rej', 'local-folder', true, true, true) ON CONFLICT DO NOTHING`)

	now := time.Now().UTC().Truncate(time.Microsecond)
	ref, _ := domain.NewFileReference("ref-rej", "epub", nil)
	cand := postgres.ImportCandidateRecord{
		ID:            "cand-rej-1",
		SourceID:      "src-rej-1",
		FileReference: ref,
		Status:        postgres.ImportCandidateStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := candRepo.Create(ctx, cand); err != nil {
		t.Fatalf("Create cand: %v", err)
	}

	if err := svc.Reject(ctx, "cand-rej-1", now); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	c, err := candRepo.Get(ctx, "cand-rej-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if c.Status != postgres.ImportCandidateStatusRejected {
		t.Fatalf("status = %q, want rejected", c.Status)
	}

	// Rejecting again returns Conflict
	err = svc.Reject(ctx, "cand-rej-1", now)
	if err == nil || domain.CategoryOf(err) != domain.Conflict {
		t.Fatalf("second reject err = %v, want Conflict", err)
	}
}
