package postgres

import (
	"findJobs/internal/ports"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProfileRepository is the outbound adapter implementing the application port.
type MasterRepository struct {
	pool *pgxpool.Pool
}

// NewProfileRepository creates the PostgreSQL adapter for profile persistence.
func NewMasterRepository(pool *pgxpool.Pool) ports.MasterRepository {
	return &MasterRepository{pool: pool}
}
