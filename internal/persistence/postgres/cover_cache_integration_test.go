//go:build integration

package postgres_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alexandryn/alexandryn/internal/persistence/postgres"
)

func TestCoverCacheRepository_SaveAndGetCover_RoundTrip(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo, err := postgres.NewCoverCacheRepository(pool, tempDir, logger)
	if err != nil {
		t.Fatalf("NewCoverCacheRepository: %v", err)
	}

	// 1x1 valid JPEG byte sequence
	validJPEG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d\x1a\x1c\x1c $.' \",#\x1c\x1c(7),01444\x1f'9=82<.342\xFF\xC0\x00\x0b\x08\x00\x01\x00\x01\x01\x01\x11\x00\xFF\xC4\x00\x1f\x00\x00\x01\x05\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\xFF\xDA\x00\x08\x01\x01\x00\x00?\x00\xbf\x00\xFF\xD9")

	// 1. Initial Get -> Miss
	_, _, _, hit, err := repo.GetCover(ctx, 12345)
	if err != nil {
		t.Fatalf("GetCover initial: %v", err)
	}
	if hit {
		t.Fatal("expected initial cache miss, got hit")
	}

	// 2. Save cover
	savedPath, err := repo.SaveCover(ctx, 12345, "image/jpeg", validJPEG)
	if err != nil {
		t.Fatalf("SaveCover: %v", err)
	}
	if savedPath == "" {
		t.Fatal("expected non-empty savedPath")
	}

	// 3. Get cover -> Hit
	filePath, contentType, isMissing, hit, err := repo.GetCover(ctx, 12345)
	if err != nil {
		t.Fatalf("GetCover after save: %v", err)
	}
	if !hit {
		t.Fatal("expected cache hit after save, got miss")
	}
	if isMissing {
		t.Fatal("expected isMissing = false, got true")
	}
	if contentType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", contentType)
	}
	if filePath != savedPath {
		t.Errorf("expected %s, got %s", savedPath, filePath)
	}

	// 4. Deleted file on disk degrades to cache miss (Reliability check)
	_ = os.Remove(savedPath)
	_, _, _, hit, err = repo.GetCover(ctx, 12345)
	if err != nil {
		t.Fatalf("GetCover after disk deletion: %v", err)
	}
	if hit {
		t.Errorf("expected cache miss when file deleted from disk, got hit")
	}
}

func TestCoverCacheRepository_MarkMissingSentinel(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo, err := postgres.NewCoverCacheRepository(pool, tempDir, logger)
	if err != nil {
		t.Fatalf("NewCoverCacheRepository: %v", err)
	}

	if err := repo.MarkMissing(ctx, 99999); err != nil {
		t.Fatalf("MarkMissing: %v", err)
	}

	_, _, isMissing, hit, err := repo.GetCover(ctx, 99999)
	if err != nil {
		t.Fatalf("GetCover for sentinel: %v", err)
	}
	if !hit {
		t.Fatal("expected sentinel hit, got miss")
	}
	if !isMissing {
		t.Fatal("expected isMissing = true")
	}
}

func TestCoverCacheRepository_LRUEviction(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	fixedTime := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	currentTime := fixedTime
	nowFunc := func() time.Time { return currentTime }

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Set maxBytes to small bound (500 bytes)
	repo, err := postgres.NewCoverCacheRepositoryWithOptions(pool, tempDir, 500, logger, nowFunc)
	if err != nil {
		t.Fatalf("NewCoverCacheRepositoryWithOptions: %v", err)
	}

	validJPEG := []byte("\xFF\xD8\xFF\xE0\x00\x10JFIF\x00\x01\x01\x01\x00`\x00`\x00\x00\xFF\xDB\x00C\x00\x08\x06\x06\x07\x06\x05\x08\x07\x07\x07\t\t\x08\n\x0c\x14\r\x0c\x0b\x0b\x0c\x19\x12\x13\x0f\x14\x1d\x1a\x1f\x1e\x1d\x1a\x1c\x1c $.' \",#\x1c\x1c(7),01444\x1f'9=82<.342\xFF\xC0\x00\x0b\x08\x00\x01\x00\x01\x01\x01\x11\x00\xFF\xC4\x00\x1f\x00\x00\x01\x05\x01\x01\x01\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\xFF\xDA\x00\x08\x01\x01\x00\x00?\x00\xbf\x00\xFF\xD9")

	// Save cover 1 at T=0 (size ~130 bytes)
	currentTime = fixedTime
	path1, err := repo.SaveCover(ctx, 1001, "image/jpeg", validJPEG)
	if err != nil {
		t.Fatalf("SaveCover 1001: %v", err)
	}

	// Save cover 2 at T=1 hour
	currentTime = fixedTime.Add(1 * time.Hour)
	_, err = repo.SaveCover(ctx, 1002, "image/jpeg", validJPEG)
	if err != nil {
		t.Fatalf("SaveCover 1002: %v", err)
	}

	// Save cover 3 at T=2 hours
	currentTime = fixedTime.Add(2 * time.Hour)
	_, err = repo.SaveCover(ctx, 1003, "image/jpeg", validJPEG)
	if err != nil {
		t.Fatalf("SaveCover 1003: %v", err)
	}

	// Save cover 4 at T=3 hours
	currentTime = fixedTime.Add(3 * time.Hour)
	_, err = repo.SaveCover(ctx, 1004, "image/jpeg", validJPEG)
	if err != nil {
		t.Fatalf("SaveCover 1004: %v", err)
	}

	// Save cover 5 at T=4 hours -> Exceeds 500 bytes bound, should evict oldest (1001)
	currentTime = fixedTime.Add(4 * time.Hour)
	_, err = repo.SaveCover(ctx, 1005, "image/jpeg", validJPEG)
	if err != nil {
		t.Fatalf("SaveCover 1005: %v", err)
	}

	// Cover 1001 should have been evicted (both DB row and disk file deleted)
	_, _, _, hit1, _ := repo.GetCover(ctx, 1001)
	if hit1 {
		t.Errorf("expected cover 1001 to be evicted, but got cache hit")
	}
	if _, err := os.Stat(path1); !os.IsNotExist(err) {
		t.Errorf("expected file %s to be deleted by eviction, stat err: %v", path1, err)
	}

	// Cover 1005 should be present
	_, _, _, hit5, _ := repo.GetCover(ctx, 1005)
	if !hit5 {
		t.Errorf("expected newest cover 1005 to be present in cache")
	}
}

func TestCoverCacheRepository_NonImageBytesRejected(t *testing.T) {
	pool := schemaTestPool(t)
	ctx := context.Background()
	tempDir := t.TempDir()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo, err := postgres.NewCoverCacheRepository(pool, tempDir, logger)
	if err != nil {
		t.Fatalf("NewCoverCacheRepository: %v", err)
	}

	nonImageData := []byte("<html><body>Not an image</body></html>")
	_, err = repo.SaveCover(ctx, 1001, "image/jpeg", nonImageData)
	if err == nil {
		t.Fatal("expected error saving non-image bytes, got nil")
	}

	// Ensure file was not created
	files, _ := filepath.Glob(filepath.Join(tempDir, "*"))
	if len(files) > 0 {
		t.Errorf("expected no files written, found: %v", files)
	}
}
