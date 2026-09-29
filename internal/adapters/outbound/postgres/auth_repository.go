package postgres

import (
	"context"
	"errors"
	"time"

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

func (r *AuthRepository) GetUserByIAMUserID(ctx context.Context, iamUserID string) (*auth.User, error) {
	const query = `SELECT id, iam_user_id, email, status FROM users WHERE iam_user_id = $1`
	row := r.pool.QueryRow(ctx, query, iamUserID)

	var user auth.User
	var id uuid.UUID
	var status string
	if err := row.Scan(&id, &user.IAMUserID, &user.Email, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, auth.ErrUserNotFound
		}
		return nil, err
	}

	user.ID = id.String()
	user.Status = status
	return &user, nil
}

func (r *AuthRepository) CreateUser(ctx context.Context, user *auth.User) error {
	const query = `INSERT INTO users (id, iam_user_id, email, status) VALUES ($1, $2, $3, $4)`
	_, err := r.pool.Exec(ctx, query, uuid.NullUUID{UUID: uuid.New(), Valid: true}, user.IAMUserID, user.Email, user.Status)
	return err
}

func (r *AuthRepository) CreateSession(ctx context.Context, userID string, iamUserID string) error {
	const query = `INSERT INTO sessions (user_id, expires_at) VALUES ($1, $2)`
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, query, uid, time.Now().Add(24*time.Hour))
	return err
}

func (r *AuthRepository) GetSession(ctx context.Context, userID string) (string, error) {
	const query = `SELECT id FROM sessions WHERE user_id = $1 AND expires_at > NOW()`
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}
	var sessionID uuid.UUID
	row := r.pool.QueryRow(ctx, query, uid)
	if err := row.Scan(&sessionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return sessionID.String(), nil
}

func (r *AuthRepository) DeleteSession(ctx context.Context, userID string) error {
	const query = `DELETE FROM sessions WHERE user_id = $1`
	uid, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, query, uid)
	return err
}