package importer_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/importer"
	"github.com/Alexandryn/alexandryn/internal/jobs"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

type fakeSourceResolver struct {
	data map[string][]byte
	err  error
}

func (r *fakeSourceResolver) Resolve(ctx context.Context, sourceID string, fileRef domain.FileReference) (io.ReadCloser, error) {
	if r.err != nil {
		return nil, r.err
	}
	d, ok := r.data[fileRef.ReferenceID]
	if !ok {
		return nil, errors.New("file not found in source")
	}
	return io.NopCloser(bytes.NewReader(d)), nil
}

type fakeCandidateRepo struct {
	candidates map[string]postgres.ImportCandidateRecord
}

func (r *fakeCandidateRepo) Create(ctx context.Context, rec postgres.ImportCandidateRecord) error {
	r.candidates[rec.ID] = rec
	return nil
}

func (r *fakeCandidateRepo) Get(ctx context.Context, id string) (postgres.ImportCandidateRecord, error) {
	c, ok := r.candidates[id]
	if !ok {
		return postgres.ImportCandidateRecord{}, &domain.Error{Category: domain.NotFound}
	}
	return c, nil
}

func (r *fakeCandidateRepo) List(ctx context.Context, sourceID *string, status *string) ([]postgres.ImportCandidateRecord, error) {
	var list []postgres.ImportCandidateRecord
	for _, c := range r.candidates {
		list = append(list, c)
	}
	return list, nil
}

func (r *fakeCandidateRepo) UpdateStatus(ctx context.Context, id string, status string, now time.Time) error {
	c, ok := r.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound}
	}
	c.Status = status
	c.UpdatedAt = now
	r.candidates[id] = c
	return nil
}

func (r *fakeCandidateRepo) UpdateExtractedAndMatches(ctx context.Context, id string, status string, extracted []byte, matches []byte, now time.Time) error {
	c, ok := r.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound}
	}
	c.Status = status
	c.ExtractedMetadata = extracted
	c.MatchCandidates = matches
	c.UpdatedAt = now
	r.candidates[id] = c
	return nil
}

func (r *fakeCandidateRepo) UpdateFailed(ctx context.Context, id string, lastError string, now time.Time) error {
	c, ok := r.candidates[id]
	if !ok {
		return &domain.Error{Category: domain.NotFound}
	}
	c.Status = postgres.ImportCandidateStatusFailed
	c.LastError = &lastError
	c.UpdatedAt = now
	r.candidates[id] = c
	return nil
}

func (r *fakeCandidateRepo) ExistsBySourceAndFileRefID(ctx context.Context, sourceID string, fileRefID string) (bool, error) {
	for _, c := range r.candidates {
		if c.SourceID == sourceID && c.FileReference.ReferenceID == fileRefID {
			return true, nil
		}
	}
	return false, nil
}

func TestJobHandler_SuccessToPending(t *testing.T) {
	ctx := context.Background()

	// Build a valid EPUB
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	wMime, _ := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	_, _ = wMime.Write([]byte("application/epub+zip"))
	wCont, _ := zw.Create("META-INF/container.xml")
	_, _ = wCont.Write([]byte(`<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`))
	wOpf, _ := zw.Create("content.opf")
	_, _ = wOpf.Write([]byte(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Ender's Game</dc:title><dc:creator>Orson Scott Card</dc:creator></metadata></package>`))
	_ = zw.Close()

	ref, _ := domain.NewFileReference("file-enders-game", "epub", nil)
	resolver := &fakeSourceResolver{
		data: map[string][]byte{
			"file-enders-game": buf.Bytes(),
		},
	}

	candRepo := &fakeCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"cand-1": {
				ID:            "cand-1",
				SourceID:      "src-1",
				FileReference: ref,
				Status:        postgres.ImportCandidateStatusQueued,
			},
		},
	}

	matcher := importer.NewMatcher(&fakeLibraryFinder{}, &fakeOpenLibrarySearcher{})
	handler := importer.NewJobHandler(resolver, candRepo, matcher, nil)

	payload, _ := json.Marshal(importer.ImportJobPayload{
		CandidateID:   "cand-1",
		SourceID:      "src-1",
		FileReference: ref,
	})

	progressReports := 0
	reportFunc := func(current, total int) {
		progressReports++
	}

	err := handler(ctx, payload, reportFunc)
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	c := candRepo.candidates["cand-1"]
	if c.Status != postgres.ImportCandidateStatusPending {
		t.Fatalf("status = %q, want pending", c.Status)
	}
	if progressReports == 0 {
		t.Error("expected progress reports during execution")
	}
}

func TestJobHandler_CorruptFileFailsPermanently(t *testing.T) {
	ctx := context.Background()

	ref, _ := domain.NewFileReference("file-corrupt", "epub", nil)
	resolver := &fakeSourceResolver{
		data: map[string][]byte{
			"file-corrupt": []byte("not a real epub or pdf or cbz"),
		},
	}

	candRepo := &fakeCandidateRepo{
		candidates: map[string]postgres.ImportCandidateRecord{
			"cand-corrupt": {
				ID:            "cand-corrupt",
				SourceID:      "src-1",
				FileReference: ref,
				Status:        postgres.ImportCandidateStatusQueued,
			},
		},
	}

	matcher := importer.NewMatcher(&fakeLibraryFinder{}, nil)
	handler := importer.NewJobHandler(resolver, candRepo, matcher, nil)

	payload, _ := json.Marshal(importer.ImportJobPayload{
		CandidateID:   "cand-corrupt",
		SourceID:      "src-1",
		FileReference: ref,
	})

	err := handler(ctx, payload, func(current, total int) {})
	if err == nil {
		t.Fatal("expected error on corrupt file")
	}

	// Must be wrapped in jobs.Permanent
	if !jobs.IsPermanent(err) {
		t.Errorf("error %v is not jobs.Permanent", err)
	}

	c := candRepo.candidates["cand-corrupt"]
	if c.Status != postgres.ImportCandidateStatusFailed {
		t.Fatalf("candidate status = %q, want failed", c.Status)
	}
	if c.LastError == nil || *c.LastError == "" {
		t.Errorf("expected lastError to be recorded")
	}
}
