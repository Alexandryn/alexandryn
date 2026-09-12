//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/adapters/crypto"
	"github.com/Alexandryn/alexandryn/internal/domain"
	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func sampleRecord(id string) postgres.SourceRecord {
	return postgres.SourceRecord{
		ID:            id,
		Label:         "Personal OPDS",
		Kind:          "opds",
		ConfigBaseURL: "https://opds.example.org/catalog",
		HealthStatus:  "unknown",
		Capabilities:  domain.SourceCapabilities{},
	}
}

func TestSourceRecordRepository_CreateGetRoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRecordRepository(pool)

	in := sampleRecord("src-1")
	in.SearchLinkURL = ""
	if err := repo.Create(ctx, in); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, "src-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Label != in.Label || got.Kind != "opds" || got.ConfigBaseURL != in.ConfigBaseURL {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.HasCredential() {
		t.Fatal("HasCredential = true for a record created without one")
	}
	if got.HealthStatus != "unknown" || got.HealthDetail != "" || got.HealthCheckedAt != nil {
		t.Fatalf("unexpected health state: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("CreatedAt not populated")
	}
}

func TestSourceRecordRepository_CredentialRoundTripThroughDB(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRecordRepository(pool)

	key := make([]byte, crypto.KeyLen)
	for i := range key {
		key[i] = byte(i)
	}
	svc, err := crypto.NewService(key)
	if err != nil {
		t.Fatalf("crypto.NewService: %v", err)
	}
	plaintext := []byte(`{"username":"reader","password":"top secret"}`)
	ct, nonce, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	rec := sampleRecord("src-cred")
	rec.CredentialCiphertext = ct
	rec.CredentialNonce = nonce
	if err := repo.Create(ctx, rec); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, "src-cred")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.HasCredential() {
		t.Fatal("HasCredential = false after storing one")
	}
	decrypted, err := svc.Decrypt(got.CredentialCiphertext, got.CredentialNonce)
	if err != nil {
		t.Fatalf("Decrypt after DB round trip: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("credential did not survive the DB round trip: %q", decrypted)
	}

	// Clearing the credential.
	if err := repo.SetCredential(ctx, "src-cred", nil, nil); err != nil {
		t.Fatalf("SetCredential(nil): %v", err)
	}
	got, _ = repo.Get(ctx, "src-cred")
	if got.HasCredential() {
		t.Fatal("credential still present after clearing")
	}
}

func TestSourceRecordRepository_ListOrdered(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRecordRepository(pool)

	base := time.Now().UTC().Add(-time.Hour)
	for i, id := range []string{"c", "a", "b"} {
		rec := sampleRecord("src-" + id)
		rec.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		if err := repo.Create(ctx, rec); err != nil {
			t.Fatalf("Create %s: %v", id, err)
		}
	}

	list, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 || list[0].ID != "src-c" || list[2].ID != "src-b" {
		t.Fatalf("List order wrong: %v", []string{list[0].ID, list[1].ID, list[2].ID})
	}
}

func TestSourceRecordRepository_UpdateConfigAndHealth(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRecordRepository(pool)

	if err := repo.Create(ctx, sampleRecord("src-u")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.UpdateConfig(ctx, "src-u", "Renamed", "", "https://new.example.org/opds"); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	if err := repo.UpdateHealth(ctx, "src-u", "reachable", "", time.Now().UTC(),
		domain.SourceCapabilities{CanList: true, CanSearch: true, CanDownload: true},
		"https://new.example.org/opds/search?q={searchTerms}"); err != nil {
		t.Fatalf("UpdateHealth: %v", err)
	}

	got, err := repo.Get(ctx, "src-u")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Label != "Renamed" || got.ConfigBaseURL != "https://new.example.org/opds" {
		t.Fatalf("config not updated: %+v", got)
	}
	if got.HealthStatus != "reachable" || !got.Capabilities.CanSearch || got.HealthCheckedAt == nil {
		t.Fatalf("health not updated: %+v", got)
	}
	if got.SearchLinkURL != "https://new.example.org/opds/search?q={searchTerms}" {
		t.Fatalf("search link not persisted: %q", got.SearchLinkURL)
	}

	// Transition back to unreachable with a detail.
	if err := repo.UpdateHealth(ctx, "src-u", "unreachable", "auth-rejected", time.Now().UTC(),
		domain.SourceCapabilities{}, ""); err != nil {
		t.Fatalf("UpdateHealth unreachable: %v", err)
	}
	got, _ = repo.Get(ctx, "src-u")
	if got.HealthStatus != "unreachable" || got.HealthDetail != "auth-rejected" {
		t.Fatalf("transition to unreachable failed: %+v", got)
	}
}

func TestSourceRecordRepository_NotFound(t *testing.T) {
	pool := schemaTestPool(t)
	repo := postgres.NewSourceRecordRepository(pool)

	if _, err := repo.Get(context.Background(), "nope"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("Get: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if err := repo.UpdateConfig(context.Background(), "nope", "x", "", ""); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("UpdateConfig: category = %v, want NotFound", domain.CategoryOf(err))
	}
	if err := repo.UpdateHealth(context.Background(), "nope", "reachable", "", time.Now(), domain.SourceCapabilities{}, ""); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("UpdateHealth: category = %v, want NotFound", domain.CategoryOf(err))
	}
}

func TestSourceRecordRepository_CountWithCredential(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRecordRepository(pool)

	plain := sampleRecord("src-plain")
	withCred := sampleRecord("src-cred")
	withCred.CredentialCiphertext = []byte{1, 2, 3}
	withCred.CredentialNonce = []byte{4, 5, 6}
	if err := repo.Create(ctx, plain); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, withCred); err != nil {
		t.Fatal(err)
	}

	n, err := repo.CountWithCredential(ctx)
	if err != nil {
		t.Fatalf("CountWithCredential: %v", err)
	}
	if n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

// Concurrent health-probe writebacks against one row: last-writer-wins,
// no deadlock, row stays consistent under concurrent execution.
func TestSourceRecordRepository_ConcurrentHealthUpdates(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	repo := postgres.NewSourceRecordRepository(pool)
	if err := repo.Create(ctx, sampleRecord("src-race")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status, detail := "reachable", ""
			if i%2 == 0 {
				status, detail = "unreachable", "timeout"
			}
			errCh <- repo.UpdateHealth(ctx, "src-race", status, detail, time.Now().UTC(),
				domain.SourceCapabilities{CanList: true, CanDownload: true}, "")
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent UpdateHealth: %v", err)
		}
	}

	got, err := repo.Get(ctx, "src-race")
	if err != nil {
		t.Fatalf("Get after race: %v", err)
	}
	if got.HealthStatus != "reachable" && got.HealthStatus != "unreachable" {
		t.Fatalf("row left in an inconsistent state: %+v", got)
	}
	if got.HealthStatus == "unreachable" && got.HealthDetail != "timeout" {
		t.Fatalf("unreachable without its detail: %+v", got)
	}
	if got.HealthStatus == "reachable" && got.HealthDetail != "" {
		t.Fatalf("reachable with a stale detail: %+v", got)
	}
}

// The DELETE path stays with domain.SourceRemovalService (atomic
// cascade). This confirms a row created via SourceRecordRepository is
// removed by that service, exercising both types against one table.
func TestSourceRecordRepository_RemovalViaService(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	recRepo := postgres.NewSourceRecordRepository(pool)
	if err := recRepo.Create(ctx, sampleRecord("src-del")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	svc := domain.NewSourceRemovalService(
		postgres.NewSourceRepository(pool),
		postgres.NewSourceOfferingRepository(pool),
		postgres.NewTransactor(pool),
	)
	if _, _, err := svc.Remove(ctx, "src-del", time.Now().UTC()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := recRepo.Get(ctx, "src-del"); domain.CategoryOf(err) != domain.NotFound {
		t.Fatalf("record still present after service removal: %v", err)
	}
}
