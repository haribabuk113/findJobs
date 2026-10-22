package masters

import (
	"context"
	models "findJobs/internal/domain/master"
)

type MasterService interface {
	// GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error)
	// UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error)
	SearchMaster(ctx context.Context, kind models.MasterKind, q string, limit int) ([]models.MasterEntryResponse, error)
	CreateMaster(ctx context.Context, kind models.MasterKind, candidateID int64, req models.MasterNameRequest) (*models.MasterEntryResponse, error)
	ListPending(ctx context.Context, kind models.MasterKind) ([]models.MasterEntryResponse, error)
	Approve(ctx context.Context, kind models.MasterKind, id int64) error
	Rename(ctx context.Context, kind models.MasterKind, id int64, req models.MasterNameRequest) error
	Delete(ctx context.Context, kind models.MasterKind, id int64) error

	SearchCompany(ctx context.Context, q string) ([]models.CompanyResponse, error)
	CreateCompany(ctx context.Context, req models.CompanyRequest) (*models.CompanyResponse, error)
	UpdateCompany(ctx context.Context, id int64, req models.CompanyRequest) error
	DeleteCompany(ctx context.Context, id int64) error

	ListLocation(ctx context.Context, q models.LocationListQuery) ([]models.LocationResponse, error)
	CreateLocation(ctx context.Context, req models.LocationRequest) (*models.LocationResponse, error)
	UpdateLocation(ctx context.Context, id int64, req models.LocationRequest) error
	DeleteLocation(ctx context.Context, id int64) error

	GetFx(ctx context.Context, date string) ([]models.FxRateResponse, error)
	PutFx(ctx context.Context, req models.PutFxRatesRequest) error
}
