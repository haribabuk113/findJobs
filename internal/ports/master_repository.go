package ports

import (
	"context"
	models "findJobs/internal/domain/master"
)

type MasterRepository interface {
	// GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error)
	// UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) error

	SearchMaster(ctx context.Context, kind models.MasterKind, q string, limit int) ([]models.MasterEntry, error)
	ListPending(ctx context.Context, kind models.MasterKind) ([]models.MasterEntry, error)
	FindOrCreatePending(ctx context.Context, kind models.MasterKind, name string, candidateID int64) (*models.MasterEntry, error)
	Approve(ctx context.Context, kind models.MasterKind, id int64) error
	Rename(ctx context.Context, kind models.MasterKind, id int64, name string) error
	Delete(ctx context.Context, kind models.MasterKind, id int64) error

	SearchCompany(ctx context.Context, q string, limit int) ([]models.Company, error)
	CreateCompany(ctx context.Context, name string, isRegistered bool) (*models.Company, error)
	UpdateCompany(ctx context.Context, c models.Company) error
	DeleteCompany(ctx context.Context, id int64) error

	ListLocation(ctx context.Context, f models.LocationFilter, limit int) ([]models.Location, error)
	CreateLocation(ctx context.Context, l models.Location) (*models.Location, error)
	UpdateLocation(ctx context.Context, l models.Location) error
	DeleteLocation(ctx context.Context, id int64) error

	LatestRate(ctx context.Context, currency string) (float64, error)
	FxLatest(ctx context.Context) ([]models.FxRate, error)
	FxByDate(ctx context.Context, date string) ([]models.FxRate, error)
	UpsertFx(ctx context.Context, date string, rates map[string]float64) error
}
