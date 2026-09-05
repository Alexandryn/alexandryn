package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// PairingSessionRepository implements domain.PairingSessionRepository
// using PostgreSQL with AES-256-GCM code encryption at rest and blind
// HMAC indexing (backend-network-api.md FR-7 / ADR 0028 §6).
type PairingSessionRepository struct {
	pool     *pgxpool.Pool
	aead     cipher.AEAD
	indexKey []byte
}

func NewPairingSessionRepository(pool *pgxpool.Pool, encKey []byte, indexKey []byte) (*PairingSessionRepository, error) {
	if len(encKey) != 32 {
		return nil, fmt.Errorf("pairing session repo: encKey must be 32 bytes, got %d", len(encKey))
	}
	if len(indexKey) != 32 {
		return nil, fmt.Errorf("pairing session repo: indexKey must be 32 bytes, got %d", len(indexKey))
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, fmt.Errorf("pairing session repo: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("pairing session repo: gcm: %w", err)
	}

	idxCopy := make([]byte, len(indexKey))
	copy(idxCopy, indexKey)

	return &PairingSessionRepository{
		pool:     pool,
		aead:     gcm,
		indexKey: idxCopy,
	}, nil
}

var _ domain.PairingSessionRepository = (*PairingSessionRepository)(nil)

// CodeIndex computes the blind HMAC-SHA256 index of the normalized code
// under pairing-code-index-v1.
func (r *PairingSessionRepository) CodeIndex(code domain.PairingCode) []byte {
	h := hmac.New(sha256.New, r.indexKey)
	h.Write([]byte(code.Normalized()))
	return h.Sum(nil)
}

func (r *PairingSessionRepository) encryptCode(code domain.PairingCode) ([]byte, error) {
	nonce := make([]byte, r.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return r.aead.Seal(nonce, nonce, []byte(code.Normalized()), nil), nil
}

func (r *PairingSessionRepository) decryptCode(ciphertext []byte) (domain.PairingCode, error) {
	nonceSize := r.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return domain.PairingCode{}, errors.New("ciphertext too short")
	}
	nonce := ciphertext[:nonceSize]
	data := ciphertext[nonceSize:]
	plaintext, err := r.aead.Open(nil, nonce, data, nil)
	if err != nil {
		return domain.PairingCode{}, err
	}
	return domain.NewPairingCode(string(plaintext))
}

const pairingSessionUpsertSQL = `INSERT INTO pairing_sessions (
		id, initiated_by, code_ciphertext, code_index, state,
		created_at, expires_at, device_id, initiator_ip
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, '')::inet
	) ON CONFLICT (id) DO UPDATE SET
		state = EXCLUDED.state,
		expires_at = EXCLUDED.expires_at,
		device_id = EXCLUDED.device_id`

func (r *PairingSessionRepository) Save(ctx context.Context, s *domain.PairingSession) error {
	return r.SaveWithInitiatorIP(ctx, s, "")
}

func (r *PairingSessionRepository) SaveWithInitiatorIP(ctx context.Context, s *domain.PairingSession, ip string) error {
	ciphertext, err := r.encryptCode(s.Code())
	if err != nil {
		return &domain.Error{Category: domain.Internal, Message: "failed to encrypt pairing code", Err: err}
	}
	codeIndex := r.CodeIndex(s.Code())

	var devID *string
	if s.DeviceID() != nil {
		val := string(*s.DeviceID())
		devID = &val
	}

	exec := executorFrom(ctx, r.pool)
	_, err = exec.Exec(ctx, pairingSessionUpsertSQL,
		string(s.ID()),
		string(s.InitiatedBy()),
		ciphertext,
		codeIndex,
		string(s.State()),
		s.CreatedAt(),
		s.ExpiresAt(),
		devID,
		ip,
	)
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

const pairingSessionSelectColumns = `id, initiated_by, code_ciphertext, state, created_at, expires_at, device_id`

const pairingSessionFindByIDSQL = `SELECT ` + pairingSessionSelectColumns + `
	FROM pairing_sessions WHERE id = $1`

func (r *PairingSessionRepository) FindByID(ctx context.Context, id domain.PairingSessionID) (*domain.PairingSession, error) {
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, pairingSessionFindByIDSQL, string(id))
	return r.scanSession(row)
}

const pairingSessionFindByCodeIndexSQL = `SELECT ` + pairingSessionSelectColumns + `
	FROM pairing_sessions WHERE code_index = $1`

func (r *PairingSessionRepository) FindByCodeIndex(ctx context.Context, codeIndex []byte) (*domain.PairingSession, error) {
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, pairingSessionFindByCodeIndexSQL, codeIndex)
	return r.scanSession(row)
}

const pairingSessionFindPendingForUpdateSQL = `SELECT ` + pairingSessionSelectColumns + `
	FROM pairing_sessions
	WHERE code_index = $1 AND state = 'pending' AND expires_at > $2
	FOR UPDATE`

func (r *PairingSessionRepository) FindPendingByCodeIndexForUpdate(ctx context.Context, codeIndex []byte, now time.Time) (*domain.PairingSession, error) {
	if _, ok := ctx.Value(txContextKey{}).(pgx.Tx); !ok {
		return nil, &domain.Error{Category: domain.Internal, Message: "FindPendingByCodeIndexForUpdate requires an active transaction"}
	}
	exec := executorFrom(ctx, r.pool)
	row := exec.QueryRow(ctx, pairingSessionFindPendingForUpdateSQL, codeIndex, now)
	return r.scanSession(row)
}

func (r *PairingSessionRepository) scanSession(row pgx.Row) (*domain.PairingSession, error) {
	var id, initiatedBy, stateStr string
	var codeCiphertext []byte
	var createdAt, expiresAt time.Time
	var deviceIDStr *string

	err := row.Scan(&id, &initiatedBy, &codeCiphertext, &stateStr, &createdAt, &expiresAt, &deviceIDStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
		}
		return nil, TranslateError(err)
	}

	code, err := r.decryptCode(codeCiphertext)
	if err != nil {
		return nil, &domain.Error{Category: domain.Internal, Message: "could not decrypt stored pairing code", Err: err}
	}

	var devID *domain.DeviceID
	if deviceIDStr != nil {
		d := domain.DeviceID(*deviceIDStr)
		devID = &d
	}

	return domain.RehydratePairingSession(
		domain.PairingSessionID(id),
		domain.UserID(initiatedBy),
		code,
		domain.PairingState(stateStr),
		createdAt,
		expiresAt,
		devID,
	)
}

const pairingSessionDeleteSQL = `DELETE FROM pairing_sessions WHERE id = $1`

func (r *PairingSessionRepository) Delete(ctx context.Context, id domain.PairingSessionID) error {
	exec := executorFrom(ctx, r.pool)
	tag, err := exec.Exec(ctx, pairingSessionDeleteSQL, string(id))
	if err != nil {
		return TranslateError(err)
	}
	if tag.RowsAffected() == 0 {
		return &domain.Error{Category: domain.NotFound, Message: "pairing session not found"}
	}
	return nil
}
