package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/admin"
	"findJobs/internal/ports"
)

// SubscriptionRepository is the PostgreSQL adapter for candidate premium
// subscriptions.
type SubscriptionRepository struct {
	pool *pgxpool.Pool
}

// NewSubscriptionRepository creates the PostgreSQL adapter for subscriptions.
func NewSubscriptionRepository(pool *pgxpool.Pool) ports.SubscriptionRepository {
	return &SubscriptionRepository{pool: pool}
}

func (r *SubscriptionRepository) Create(ctx context.Context, s *admin.Subscription) error {
	// Set now for timestamps
	now := time.Now()
	s.CreatedAt = now
	s.StartedAt = now
	s.ExpiresAt = now.Add(time.Hour * 24 * time.Duration(s.DurationDays))
	if err := s.Validate(); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO subscriptions (id, user_id, duration_days, started_at, expires_at, status, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, s.ID, s.UserID, s.DurationDays, s.StartedAt, s.ExpiresAt, s.Status, s.CreatedBy, s.CreatedAt)
	return err
}

func (r *SubscriptionRepository) Extend(ctx context.Context, id string, durationDays int) error {
	if durationDays < 1 || durationDays > 365 {
		return &admin.ValidationError{Field: "duration_days", Message: "duration_days must be between 1 and 365"}
	}

	now := time.Now()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE subscriptions SET expires_at = $1 + INTERVAL 'P' || $2 || 'D', created_at = NOW() WHERE id = $3`, now, durationDays, id)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *SubscriptionRepository) ListByUser(ctx context.Context, userID string, page, pageSize int) (*admin.SubscriptionPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	from := `FROM subscriptions WHERE user_id = $1`
	var totalRows int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) `+from, userID).Scan(&totalRows)
	if err != nil {
		return nil, err
	}

	query := `SELECT id, user_id, duration_days, started_at, expires_at, status, created_by, created_at ` + from + `
		ORDER BY started_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.pool.Query(ctx, query, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := &admin.SubscriptionPage{Subscriptions: make([]admin.Subscription, 0, pageSize), Page: page, PageSize: pageSize, Total: totalRows}
	for rows.Next() {
		var s admin.Subscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.DurationDays, &s.StartedAt, &s.ExpiresAt, &s.Status, &s.CreatedBy, &s.CreatedAt); err != nil {
			return nil, err
		}
		result.Subscriptions = append(result.Subscriptions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SubscriptionRepository) ExpireSubscriptions(ctx context.Context) ([]string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Find active subscriptions to expire
	rows, err := tx.Query(ctx, `SELECT user_id FROM subscriptions WHERE status = 'active' AND expires_at < NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	userIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	if len(userIDs) == 0 {
		return userIDs, tx.Commit(ctx)
	}

	// Expire them
	_, err = tx.Exec(ctx, `UPDATE subscriptions SET status = 'expired' WHERE status = 'active' AND expires_at < NOW()`)
	if err != nil {
		return nil, err
	}
	return userIDs, tx.Commit(ctx)
}
