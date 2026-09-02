package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Alexandryn/alexandryn/internal/domain"
)

// LibraryRepository implements domain.LibraryRepository.
type LibraryRepository struct {
	pool *pgxpool.Pool
}

func NewLibraryRepository(pool *pgxpool.Pool) *LibraryRepository {
	return &LibraryRepository{pool: pool}
}

var _ domain.LibraryRepository = (*LibraryRepository)(nil)

func (r *LibraryRepository) FindByID(ctx context.Context, id domain.LibraryID) (*domain.Library, error) {
	exec := executorFrom(ctx, r.pool)
	var name, description string
	var allowReaderUploads bool
	var createdAt, updatedAt time.Time

	err := exec.QueryRow(ctx, `
		SELECT name, description, allow_reader_uploads, created_at, updated_at
		FROM libraries
		WHERE id = $1
	`, string(id)).Scan(&name, &description, &allowReaderUploads, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "library not found"}
		}
		return nil, TranslateError(err)
	}

	return domain.NewLibrary(id, name, description, allowReaderUploads, createdAt, updatedAt)
}

func (r *LibraryRepository) FindAll(ctx context.Context) ([]*domain.Library, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, `
		SELECT id, name, description, allow_reader_uploads, created_at, updated_at
		FROM libraries
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.Library
	for rows.Next() {
		var id, name, description string
		var allowReaderUploads bool
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &name, &description, &allowReaderUploads, &createdAt, &updatedAt); err != nil {
			return nil, TranslateError(err)
		}

		lib, err := domain.NewLibrary(domain.LibraryID(id), name, description, allowReaderUploads, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, lib)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *LibraryRepository) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.Library, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, `
		SELECT l.id, l.name, l.description, l.allow_reader_uploads, l.created_at, l.updated_at
		FROM libraries l
		JOIN library_memberships lm ON l.id = lm.library_id
		WHERE lm.user_id = $1
		ORDER BY l.name ASC
	`, string(userID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.Library
	for rows.Next() {
		var id, name, description string
		var allowReaderUploads bool
		var createdAt, updatedAt time.Time

		if err := rows.Scan(&id, &name, &description, &allowReaderUploads, &createdAt, &updatedAt); err != nil {
			return nil, TranslateError(err)
		}

		lib, err := domain.NewLibrary(domain.LibraryID(id), name, description, allowReaderUploads, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, lib)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *LibraryRepository) Save(ctx context.Context, l *domain.Library) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO libraries (id, name, description, allow_reader_uploads, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			allow_reader_uploads = EXCLUDED.allow_reader_uploads,
			updated_at = EXCLUDED.updated_at
	`, string(l.ID()), l.Name(), l.Description(), l.AllowReaderUploads(), l.CreatedAt(), l.UpdatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *LibraryRepository) Delete(ctx context.Context, id domain.LibraryID) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `DELETE FROM libraries WHERE id = $1`, string(id))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// LibraryMembershipRepository implements domain.LibraryMembershipRepository.
type LibraryMembershipRepository struct {
	pool *pgxpool.Pool
}

func NewLibraryMembershipRepository(pool *pgxpool.Pool) *LibraryMembershipRepository {
	return &LibraryMembershipRepository{pool: pool}
}

var _ domain.LibraryMembershipRepository = (*LibraryMembershipRepository)(nil)

func (r *LibraryMembershipRepository) FindMembership(ctx context.Context, libraryID domain.LibraryID, userID domain.UserID) (*domain.LibraryMembership, error) {
	exec := executorFrom(ctx, r.pool)
	var id, roleStr string
	var createdAt time.Time

	err := exec.QueryRow(ctx, `
		SELECT id, role, created_at
		FROM library_memberships
		WHERE library_id = $1 AND user_id = $2
	`, string(libraryID), string(userID)).Scan(&id, &roleStr, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "membership not found"}
		}
		return nil, TranslateError(err)
	}

	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return nil, err
	}
	return domain.NewLibraryMembership(domain.LibraryMembershipID(id), libraryID, userID, role, createdAt)
}

func (r *LibraryMembershipRepository) FindByLibrary(ctx context.Context, libraryID domain.LibraryID) ([]*domain.LibraryMembership, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, `
		SELECT id, user_id, role, created_at
		FROM library_memberships
		WHERE library_id = $1
		ORDER BY created_at ASC
	`, string(libraryID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.LibraryMembership
	for rows.Next() {
		var id, userID, roleStr string
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &roleStr, &createdAt); err != nil {
			return nil, TranslateError(err)
		}

		role, err := domain.ParseRole(roleStr)
		if err != nil {
			return nil, err
		}
		mem, err := domain.NewLibraryMembership(domain.LibraryMembershipID(id), libraryID, domain.UserID(userID), role, createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, mem)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *LibraryMembershipRepository) FindByUser(ctx context.Context, userID domain.UserID) ([]*domain.LibraryMembership, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, `
		SELECT id, library_id, role, created_at
		FROM library_memberships
		WHERE user_id = $1
		ORDER BY created_at ASC
	`, string(userID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.LibraryMembership
	for rows.Next() {
		var id, libraryID, roleStr string
		var createdAt time.Time

		if err := rows.Scan(&id, &libraryID, &roleStr, &createdAt); err != nil {
			return nil, TranslateError(err)
		}

		role, err := domain.ParseRole(roleStr)
		if err != nil {
			return nil, err
		}
		mem, err := domain.NewLibraryMembership(domain.LibraryMembershipID(id), domain.LibraryID(libraryID), userID, role, createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, mem)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *LibraryMembershipRepository) Save(ctx context.Context, m *domain.LibraryMembership) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO library_memberships (id, library_id, user_id, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (library_id, user_id) DO UPDATE SET
			role = EXCLUDED.role
	`, string(m.ID()), string(m.LibraryID()), string(m.UserID()), string(m.Role()), m.CreatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *LibraryMembershipRepository) Delete(ctx context.Context, libraryID domain.LibraryID, userID domain.UserID) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		DELETE FROM library_memberships
		WHERE library_id = $1 AND user_id = $2
	`, string(libraryID), string(userID))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

// LibraryInvitationRepository implements domain.LibraryInvitationRepository.
type LibraryInvitationRepository struct {
	pool *pgxpool.Pool
}

func NewLibraryInvitationRepository(pool *pgxpool.Pool) *LibraryInvitationRepository {
	return &LibraryInvitationRepository{pool: pool}
}

var _ domain.LibraryInvitationRepository = (*LibraryInvitationRepository)(nil)

func (r *LibraryInvitationRepository) FindByID(ctx context.Context, id domain.LibraryInvitationID) (*domain.LibraryInvitation, error) {
	exec := executorFrom(ctx, r.pool)
	var libraryID, email, roleStr, tokenHash, createdBy string
	var expiresAt, createdAt time.Time
	var usedAt *time.Time

	err := exec.QueryRow(ctx, `
		SELECT library_id, email, role, token_hash, created_by, expires_at, used_at, created_at
		FROM library_invitations
		WHERE id = $1
	`, string(id)).Scan(&libraryID, &email, &roleStr, &tokenHash, &createdBy, &expiresAt, &usedAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "invitation not found"}
		}
		return nil, TranslateError(err)
	}

	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateLibraryInvitation(id, domain.LibraryID(libraryID), email, role, tokenHash, domain.UserID(createdBy), expiresAt, usedAt, createdAt), nil
}

func (r *LibraryInvitationRepository) FindByTokenHash(ctx context.Context, hash string) (*domain.LibraryInvitation, error) {
	exec := executorFrom(ctx, r.pool)
	var id, libraryID, email, roleStr, createdBy string
	var expiresAt, createdAt time.Time
	var usedAt *time.Time

	err := exec.QueryRow(ctx, `
		SELECT id, library_id, email, role, created_by, expires_at, used_at, created_at
		FROM library_invitations
		WHERE token_hash = $1
	`, hash).Scan(&id, &libraryID, &email, &roleStr, &createdBy, &expiresAt, &usedAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &domain.Error{Category: domain.NotFound, Message: "invitation not found"}
		}
		return nil, TranslateError(err)
	}

	role, err := domain.ParseRole(roleStr)
	if err != nil {
		return nil, err
	}
	return domain.RehydrateLibraryInvitation(domain.LibraryInvitationID(id), domain.LibraryID(libraryID), email, role, hash, domain.UserID(createdBy), expiresAt, usedAt, createdAt), nil
}

func (r *LibraryInvitationRepository) FindByLibrary(ctx context.Context, libraryID domain.LibraryID) ([]*domain.LibraryInvitation, error) {
	exec := executorFrom(ctx, r.pool)
	rows, err := exec.Query(ctx, `
		SELECT id, email, role, token_hash, created_by, expires_at, used_at, created_at
		FROM library_invitations
		WHERE library_id = $1
		ORDER BY created_at DESC
	`, string(libraryID))
	if err != nil {
		return nil, TranslateError(err)
	}
	defer rows.Close()

	var result []*domain.LibraryInvitation
	for rows.Next() {
		var id, email, roleStr, tokenHash, createdBy string
		var expiresAt, createdAt time.Time
		var usedAt *time.Time

		if err := rows.Scan(&id, &email, &roleStr, &tokenHash, &createdBy, &expiresAt, &usedAt, &createdAt); err != nil {
			return nil, TranslateError(err)
		}

		role, err := domain.ParseRole(roleStr)
		if err != nil {
			return nil, err
		}
		inv := domain.RehydrateLibraryInvitation(domain.LibraryInvitationID(id), libraryID, email, role, tokenHash, domain.UserID(createdBy), expiresAt, usedAt, createdAt)
		result = append(result, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, TranslateError(err)
	}
	return result, nil
}

func (r *LibraryInvitationRepository) Save(ctx context.Context, inv *domain.LibraryInvitation) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `
		INSERT INTO library_invitations (id, library_id, email, role, token_hash, created_by, expires_at, used_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			used_at = EXCLUDED.used_at
	`, string(inv.ID()), string(inv.LibraryID()), inv.Email(), string(inv.Role()), inv.TokenHash(), string(inv.CreatedBy()), inv.ExpiresAt(), inv.UsedAt(), inv.CreatedAt())
	if err != nil {
		return TranslateError(err)
	}
	return nil
}

func (r *LibraryInvitationRepository) Delete(ctx context.Context, id domain.LibraryInvitationID) error {
	exec := executorFrom(ctx, r.pool)
	_, err := exec.Exec(ctx, `DELETE FROM library_invitations WHERE id = $1`, string(id))
	if err != nil {
		return TranslateError(err)
	}
	return nil
}
