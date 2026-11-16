package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"findJobs/internal/domain/admin"
	"findJobs/internal/ports"
)

// AuditRepository is the PostgreSQL adapter for the God Mode audit trail.
type AuditRepository struct {
	pool *pgxpool.Pool
}

// NewAuditRepository creates the PostgreSQL adapter for the audit log.
func NewAuditRepository(pool *pgxpool.Pool) ports.AuditRepository {
	return &AuditRepository{pool: pool}
}

func (r *AuditRepository) Append(ctx context.Context, a *admin.AuditLog) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (actor_id, actor_type, action, target_id, target_type, changes_json)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ActorID, a.ActorType, a.Action, a.TargetID, a.TargetType, a.Changes)
	return err
}

func (r *AuditRepository) List(ctx context.Context, limit int) ([]admin.AuditLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, actor_id, actor_type, action, target_id, target_type, changes_json, created_at
		FROM audit_log ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]admin.AuditLog, 0, limit)
	for rows.Next() {
		var a admin.AuditLog
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorType, &a.Action, &a.TargetID, &a.TargetType, &a.Changes, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}