package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// UserRepository implements domain.UserRepository.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

var _ domain.UserRepository = (*UserRepository)(nil)

func (r *UserRepository) FindByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	exec := executorFrom(ctx, r.pool)
	var username, email, roleStr string
	var createdAt, updatedAt time.Time

	err := exec.QueryRow(ctx, `SELECT username, email, role, created_at, updated_at FROM users WHERE id = $1`, string(id)).
		Scan(&username, &email, &roleStr, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "user not found"}
		}
		return nil, TranslateError(err)
	}

	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return nil, err
	}
	return domain.NewUser(id, username, email, role, createdAt, updatedAt)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	exec := executorFrom(ctx, r.pool)
	var id, username, roleStr string
	var createdAt, updatedAt time.Time

	err := exec.QueryRow(ctx, `SELECT id, username, role, created_at, updated_at FROM users WHERE LOWER(email) = LOWER($1)`, email).
		Scan(&id, &username, &roleStr, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "user not found"}
		}
		return nil, TranslateError(err)
	}

	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return nil, err
	}
	return domain.NewUser(domain.UserID(id), username, email, role, createdAt, updatedAt)
}

func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	exec := executorFrom(ctx, r.pool)
	var id, email, roleStr string
	var createdAt, updatedAt time.Time

	err := exec.QueryRow(ctx, `SELECT id, email, role, created_at, updated_at FROM users WHERE LOWER(username) = LOWER($1)`, username).
		Scan(&id, &email, &roleStr, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "user not found"}
		}
		return nil, TranslateError(err)
	}

	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return nil, err
	}
	return domain.NewUser(domain.UserID(id), username, email, role, createdAt, updatedAt)
}

func (r *UserRepository) CountUsers(ctx context.Context) (int, error) {
	exec := executorFrom(ctx, r.pool)
	var count int
	err := exec.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, TranslateError(err)
	}
	return count, nil
}

func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO users (id, username, email, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			email = EXCLUDED.email,
			role = EXCLUDED.role,
			updated_at = EXCLUDED.updated_at
	`, string(u.ID()), u.Username(), u.Email(), string(u.Role()), u.CreatedAt(), u.UpdatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// CredentialRepository implements domain.CredentialRepository.
type CredentialRepository struct {
	pool *pgxpool.Pool
}

func NewCredentialRepository(pool *pgxpool.Pool) *CredentialRepository {
	return &CredentialRepository{pool: pool}
}

var _ domain.CredentialRepository = (*CredentialRepository)(nil)

func (r *CredentialRepository) FindByUserID(ctx context.Context, userID domain.UserID) (*domain.UserCredentials, error) {
	exec := executorFrom(ctx, r.pool)
	var hash string
	var updatedAt time.Time

	err := exec.QueryRow(ctx, `SELECT password_hash, updated_at FROM user_credentials WHERE user_id = $1`, string(userID)).
		Scan(&hash, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "credentials not found"}
		}
		return nil, TranslateError(err)
	}
	return domain.NewUserCredentials(userID, hash, updatedAt)
}

func (r *CredentialRepository) Save(ctx context.Context, creds *domain.UserCredentials) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO user_credentials (user_id, password_hash, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			updated_at = EXCLUDED.updated_at
	`, string(creds.UserID()), creds.PasswordHash(), creds.UpdatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// RefreshTokenRepository implements domain.RefreshTokenRepository.
type RefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{pool: pool}
}

var _ domain.RefreshTokenRepository = (*RefreshTokenRepository)(nil)

func (r *RefreshTokenRepository) FindByTokenHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	exec := executorFrom(ctx, r.pool)
	var id, userID string
	var expiresAt, createdAt time.Time
	var revokedAt *time.Time

	err := exec.QueryRow(ctx, `SELECT id, user_id, expires_at, revoked_at, created_at FROM user_refresh_tokens WHERE token_hash = $1`, hash).
		Scan(&id, &userID, &expiresAt, &revokedAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "refresh token not found"}
		}
		return nil, TranslateError(err)
	}
	return domain.RehydrateRefreshToken(domain.RefreshTokenID(id), domain.UserID(userID), hash, expiresAt, revokedAt, createdAt), nil
}

func (r *RefreshTokenRepository) Save(ctx context.Context, rt *domain.RefreshToken) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO user_refresh_tokens (id, user_id, token_hash, expires_at, revoked_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			revoked_at = EXCLUDED.revoked_at
	`, string(rt.ID()), string(rt.UserID()), rt.TokenHash(), rt.ExpiresAt(), rt.RevokedAt(), rt.CreatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID domain.UserID, now time.Time) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `UPDATE user_refresh_tokens SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`, now, string(userID))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// MFARepository implements domain.MFARepository.
type MFARepository struct {
	pool *pgxpool.Pool
}

func NewMFARepository(pool *pgxpool.Pool) *MFARepository {
	return &MFARepository{pool: pool}
}

var _ domain.MFARepository = (*MFARepository)(nil)

func (r *MFARepository) FindByUserID(ctx context.Context, userID domain.UserID) (*domain.TOTPSettings, error) {
	exec := executorFrom(ctx, r.pool)
	var secret []byte
	var recoveryHashes []string
	var enabled bool
	var confirmedAt *time.Time
	var createdAt time.Time

	err := exec.QueryRow(ctx, `SELECT encrypted_secret, recovery_code_hashes, enabled, confirmed_at, created_at FROM mfa_totp_settings WHERE user_id = $1`, string(userID)).
		Scan(&secret, &recoveryHashes, &enabled, &confirmedAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "MFA settings not found"}
		}
		return nil, TranslateError(err)
	}
	return domain.NewTOTPSettings(userID, secret, recoveryHashes, enabled, confirmedAt, createdAt)
}

func (r *MFARepository) Save(ctx context.Context, s *domain.TOTPSettings) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO mfa_totp_settings (user_id, encrypted_secret, recovery_code_hashes, enabled, confirmed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			encrypted_secret = EXCLUDED.encrypted_secret,
			recovery_code_hashes = EXCLUDED.recovery_code_hashes,
			enabled = EXCLUDED.enabled,
			confirmed_at = EXCLUDED.confirmed_at
	`, string(s.UserID()), s.EncryptedSecret(), s.RecoveryCodeHashes(), s.IsEnabled(), s.ConfirmedAt(), s.CreatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *MFARepository) Delete(ctx context.Context, userID domain.UserID) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `DELETE FROM mfa_totp_settings WHERE user_id = $1`, string(userID))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// PasswordResetRepository implements domain.PasswordResetRepository.
type PasswordResetRepository struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{pool: pool}
}

var _ domain.PasswordResetRepository = (*PasswordResetRepository)(nil)

func (r *PasswordResetRepository) FindByTokenHash(ctx context.Context, hash string) (*domain.PasswordResetToken, error) {
	exec := executorFrom(ctx, r.pool)
	var id, userID string
	var expiresAt, createdAt time.Time
	var usedAt *time.Time

	err := exec.QueryRow(ctx, `SELECT id, user_id, expires_at, used_at, created_at FROM password_resets WHERE token_hash = $1`, hash).
		Scan(&id, &userID, &expiresAt, &usedAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "password reset token not found"}
		}
		return nil, TranslateError(err)
	}
	return domain.RehydratePasswordResetToken(domain.PasswordResetID(id), domain.UserID(userID), hash, expiresAt, usedAt, createdAt), nil
}

func (r *PasswordResetRepository) Save(ctx context.Context, prt *domain.PasswordResetToken) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO password_resets (id, user_id, token_hash, expires_at, used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			used_at = EXCLUDED.used_at
	`, string(prt.ID()), string(prt.UserID()), prt.TokenHash(), prt.ExpiresAt(), prt.UsedAt(), prt.CreatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}
