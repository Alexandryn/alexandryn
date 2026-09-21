package postgres

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// DefaultCoverCacheMaxBytes is 500 MiB.
const DefaultCoverCacheMaxBytes int64 = 500 * 1024 * 1024

// CoverCacheRepository defines persistence and filesystem caching for covers.
type CoverCacheRepository interface {
	GetCover(ctx context.Context, coverID int64) (filePath string, contentType string, isMissing bool, hit bool, err error)
	SaveCover(ctx context.Context, coverID int64, contentType string, data []byte) (string, error)
	MarkMissing(ctx context.Context, coverID int64) error
}

// FilesystemCoverCacheRepository manages cover images on local disk and metadata in PostgreSQL.
type FilesystemCoverCacheRepository struct {
	pool      *pgxpool.Pool
	coversDir string
	maxBytes  int64
	logger    *slog.Logger
	now       func() time.Time
	mu        sync.Mutex
}

// NewCoverCacheRepository constructs a new FilesystemCoverCacheRepository.
func NewCoverCacheRepository(pool *pgxpool.Pool, coversDir string, logger *slog.Logger) (*FilesystemCoverCacheRepository, error) {
	return NewCoverCacheRepositoryWithOptions(pool, coversDir, DefaultCoverCacheMaxBytes, logger, time.Now)
}

// NewCoverCacheRepositoryWithOptions constructs a repository with custom disk limit and clock.
func NewCoverCacheRepositoryWithOptions(pool *pgxpool.Pool, coversDir string, maxBytes int64, logger *slog.Logger, now func() time.Time) (*FilesystemCoverCacheRepository, error) {
	if coversDir == "" {
		return nil, errors.New("coversDir cannot be empty")
	}
	if err := os.MkdirAll(coversDir, 0755); err != nil {
		return nil, fmt.Errorf("creating covers directory: %w", err)
	}

	if maxBytes <= 0 {
		maxBytes = DefaultCoverCacheMaxBytes
	}
	if now == nil {
		now = time.Now
	}

	return &FilesystemCoverCacheRepository{
		pool:      pool,
		coversDir: coversDir,
		maxBytes:  maxBytes,
		logger:    logger,
		now:       now,
	}, nil
}

var _ CoverCacheRepository = (*FilesystemCoverCacheRepository)(nil)

// GetCover checks if a cover image is cached locally or recorded as missing.
func (r *FilesystemCoverCacheRepository) GetCover(ctx context.Context, coverID int64) (string, string, bool, bool, error) {
	if coverID <= 0 {
		return "", "", false, false, &domain.Error{Category: domain.InvalidInput, Message: "cover ID must be a positive integer"}
	}

	var (
		filePath    string
		contentType string
		isMissing   bool
	)

	err := r.pool.QueryRow(ctx,
		`SELECT file_path, content_type, is_missing FROM metadata_covers WHERE cover_id = $1`,
		coverID,
	).Scan(&filePath, &contentType, &isMissing)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, false, nil // Cache miss
		}
		return "", "", false, false, &domain.Error{Category: domain.Internal, Message: "failed to query cover cache"}
	}

	if isMissing {
		return "", "", true, true, nil // Known 404 sentinel hit
	}

	// Verify file actually exists on disk (Reliability guarantee)
	if _, err := os.Stat(filePath); err != nil {
		// Stale row without underlying file -> treat as cache miss
		return "", "", false, false, nil
	}

	// Update accessed_at for LRU eviction
	now := r.now()
	_, _ = r.pool.Exec(ctx, `UPDATE metadata_covers SET accessed_at = $1 WHERE cover_id = $2`, now, coverID)

	if contentType == "" {
		contentType = "image/jpeg"
	}

	return filePath, contentType, false, true, nil
}

// SaveCover validates image format, atomically writes bytes to disk, and updates the database.
func (r *FilesystemCoverCacheRepository) SaveCover(ctx context.Context, coverID int64, contentType string, data []byte) (string, error) {
	if coverID <= 0 {
		return "", &domain.Error{Category: domain.InvalidInput, Message: "cover ID must be a positive integer"}
	}
	if len(data) == 0 {
		return "", &domain.Error{Category: domain.InvalidInput, Message: "cover data is empty"}
	}

	// Shape check (magic-byte image format validation)
	if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
		if r.logger != nil {
			r.logger.Warn("Cover image failed format validation", slog.Int64("cover_id", coverID), slog.Any("error", err))
		}
		return "", &domain.Error{Category: domain.Unavailable, Message: "invalid image format received from upstream"}
	}

	if contentType == "" {
		contentType = "image/jpeg"
	}

	// Safe filename strictly derived from numeric ID (prevents path traversal per Security considerations)
	finalPath := filepath.Join(r.coversDir, fmt.Sprintf("%d.jpg", coverID))

	// Atomic write via temp file
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	tmpPath := filepath.Join(r.coversDir, fmt.Sprintf("tmp_%d_%s.tmp", coverID, hex.EncodeToString(randBytes)))

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		if r.logger != nil {
			r.logger.Warn("Failed to write cover image to disk", slog.Int64("cover_id", coverID), slog.Any("error", err))
		}
		return "", &domain.Error{Category: domain.Internal, Message: "failed to write cover file"}
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		if r.logger != nil {
			r.logger.Warn("Failed to rename temporary cover file", slog.Int64("cover_id", coverID), slog.Any("error", err))
		}
		return "", &domain.Error{Category: domain.Internal, Message: "failed to commit cover file"}
	}

	now := r.now()
	byteSize := int64(len(data))

	_, err := r.pool.Exec(ctx,
		`INSERT INTO metadata_covers (cover_id, file_path, byte_size, content_type, is_missing, fetched_at, accessed_at)
		 VALUES ($1, $2, $3, $4, FALSE, $5, $6)
		 ON CONFLICT (cover_id) DO UPDATE SET
		   file_path = EXCLUDED.file_path,
		   byte_size = EXCLUDED.byte_size,
		   content_type = EXCLUDED.content_type,
		   is_missing = FALSE,
		   fetched_at = EXCLUDED.fetched_at,
		   accessed_at = EXCLUDED.accessed_at`,
		coverID, finalPath, byteSize, contentType, now, now,
	)
	if err != nil {
		if r.logger != nil {
			r.logger.Warn("Failed to record cover in database", slog.Int64("cover_id", coverID), slog.Any("error", err))
		}
		// A cache-write failure MUST NOT fail the request
	}

	// Trigger LRU eviction check
	r.evictIfNeeded(ctx)

	return finalPath, nil
}

// MarkMissing records a 404 sentinel in the cache.
func (r *FilesystemCoverCacheRepository) MarkMissing(ctx context.Context, coverID int64) error {
	if coverID <= 0 {
		return &domain.Error{Category: domain.InvalidInput, Message: "cover ID must be a positive integer"}
	}

	now := r.now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO metadata_covers (cover_id, file_path, byte_size, content_type, is_missing, fetched_at, accessed_at)
		 VALUES ($1, '', 0, '', TRUE, $2, $3)
		 ON CONFLICT (cover_id) DO UPDATE SET
		   file_path = EXCLUDED.file_path,
		   byte_size = EXCLUDED.byte_size,
		   content_type = EXCLUDED.content_type,
		   is_missing = TRUE,
		   fetched_at = EXCLUDED.fetched_at,
		   accessed_at = EXCLUDED.accessed_at`,
		coverID, now, now,
	)
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to mark cover as missing"}
	}
	return nil
}

// evictIfNeeded runs LRU eviction if total disk usage exceeds maxBytes.
func (r *FilesystemCoverCacheRepository) evictIfNeeded(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var totalBytes int64
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(SUM(byte_size), 0) FROM metadata_covers WHERE is_missing = FALSE`).Scan(&totalBytes)
	if err != nil || totalBytes <= r.maxBytes {
		return
	}

	rows, err := r.pool.Query(ctx,
		`SELECT cover_id, file_path, byte_size
		 FROM metadata_covers
		 WHERE is_missing = FALSE
		 ORDER BY accessed_at ASC`,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	type candidate struct {
		coverID  int64
		filePath string
		size     int64
	}
	var candidates []candidate

	for rows.Next() && totalBytes > r.maxBytes {
		var c candidate
		if err := rows.Scan(&c.coverID, &c.filePath, &c.size); err != nil {
			break
		}
		candidates = append(candidates, c)
		totalBytes -= c.size
	}
	rows.Close()

	var evictedCount int
	for _, c := range candidates {
		_ = os.Remove(c.filePath)
		_, _ = r.pool.Exec(ctx, `DELETE FROM metadata_covers WHERE cover_id = $1`, c.coverID)
		evictedCount++
	}

	if evictedCount > 0 && r.logger != nil {
		r.logger.Info("Cover cache eviction pass completed",
			slog.Int("files_removed", evictedCount),
			slog.Int64("total_bytes_remaining", totalBytes),
			slog.Int64("max_bytes", r.maxBytes),
		)
	}
}
