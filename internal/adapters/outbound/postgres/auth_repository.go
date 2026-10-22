package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/auth"
	"findJobs/internal/ports"
)

// AuthRepository is the outbound adapter for users. Sessions: session_store.go.
type AuthRepository struct {
	pool *pgxpool.Pool
}

// NewAuthRepository creates the PostgreSQL adapter for user persistence.
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

func (r *AuthRepository) UserCompanies(ctx context.Context, userID string) ([]auth.CompanyMembership, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.slug, c.name, m.role FROM company_members m JOIN companies c ON c.id = m.company_id
		WHERE m.user_id = $1 AND m.status = 'current' AND c.deleted_at IS NULL
		ORDER BY (m.role = 'admin') DESC, c.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[auth.CompanyMembership])
}
