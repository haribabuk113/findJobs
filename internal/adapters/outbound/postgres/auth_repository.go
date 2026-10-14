package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

// AuthRepository is the outbound adapter implementing the application port.
type AuthRepository struct {
	pool *pgxpool.Pool
}

// NewAuthRepository creates the PostgreSQL adapter for auth persistence.
func NewAuthRepository(pool *pgxpool.Pool) ports.AuthRepository {
	return &AuthRepository{pool: pool}
}

func (r *AuthRepository) UpsertUser(ctx context.Context, user *auth.User) (*auth.User, error) {
	const query = `
		INSERT INTO users (iam_user_id, name, email, status) VALUES ($1, $2, $3, $4)
		ON CONFLICT (iam_user_id) DO UPDATE
		SET name = EXCLUDED.name, email = EXCLUDED.email, status = EXCLUDED.status, updated_at = NOW()
		RETURNING id::text, iam_user_id, name, email, status`
	var u auth.User
	err := r.pool.QueryRow(ctx, query, user.IAMUserID, user.Name, user.Email, user.Status).
		Scan(&u.ID, &u.IAMUserID, &u.Name, &u.Email, &u.Status)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *AuthRepository) GetUserByID(ctx context.Context, userID string) (*auth.User, error) {
	const query = `SELECT id::text, iam_user_id, name, email, status FROM users WHERE id = $1`
	var u auth.User
	err := r.pool.QueryRow(ctx, query, userID).Scan(&u.ID, &u.IAMUserID, &u.Name, &u.Email, &u.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *AuthRepository) CreateSession(ctx context.Context, s *auth.Session) (string, error) {
	const query = `
		INSERT INTO sessions (user_id, refresh_token, access_token, access_expires_at, expires_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id::text`
	var id string
	err := r.pool.QueryRow(ctx, query, s.UserID, s.RefreshToken, s.AccessToken, s.AccessExpiresAt, s.ExpiresAt).Scan(&id)
	return id, err
}

// ponytail: holds a row lock across fn (which calls IAM /refresh, bounded by
// the IAM client timeout) so concurrent requests on one session never present
// the same refresh token twice — IAM treats that as theft and revokes.
func (r *AuthRepository) LockSession(ctx context.Context, sessionID string, fn func(*auth.Session) (*auth.Session, error)) (*auth.Session, error) {
	if uuid.Validate(sessionID) != nil {
		return nil, auth.ErrUnauthorized
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	const query = `
		SELECT id::text, user_id::text, refresh_token, access_token, access_expires_at, expires_at
		FROM sessions WHERE id = $1 AND expires_at > NOW() FOR UPDATE`
	var s auth.Session
	err = tx.QueryRow(ctx, query, sessionID).
		Scan(&s.ID, &s.UserID, &s.RefreshToken, &s.AccessToken, &s.AccessExpiresAt, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, auth.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	updated, err := fn(&s)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return &s, tx.Commit(ctx)
	}
	const update = `UPDATE sessions SET refresh_token = $2, access_token = $3, access_expires_at = $4, expires_at = $5 WHERE id = $1`
	if _, err := tx.Exec(ctx, update, s.ID, updated.RefreshToken, updated.AccessToken, updated.AccessExpiresAt, updated.ExpiresAt); err != nil {
		return nil, err
	}
	return updated, tx.Commit(ctx)
}

func (r *AuthRepository) DeleteSession(ctx context.Context, sessionID string) (string, error) {
	if uuid.Validate(sessionID) != nil {
		return "", nil
	}
	var refreshToken string
	err := r.pool.QueryRow(ctx, `DELETE FROM sessions WHERE id = $1 RETURNING refresh_token`, sessionID).Scan(&refreshToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return refreshToken, err
}
