package masters

import (
	"context"
	models "findJobs/internal/domain/master"
	"findJobs/internal/ports"
	utils "findJobs/internal/utils/common"
	apperror "findJobs/internal/utils/errors"
	"net/http"
	"strings"
)

type service struct {
	repo ports.MasterRepository
	// storage ports.CDNProfilePhoto
}

// NewProfileService creates a new profile service using dependency injection
func NewMasterService(repo ports.MasterRepository) MasterService {
	return &service{repo: repo}
}

// // GetProfile retrieves a user profile by ID
// func (s *service) GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error) {
// 	return s.repo.GetProfile(ctx, userID)
// }

// // UpdateProfile updates a user profile
// func (s *service) UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
// 	if err := request.Validate(); err != nil {
// 		return "", err
// 	}

// 	if err := s.repo.UpdateProfile(ctx, request); err != nil {
// 		return "", err
// 	}

// 	return "Profile updated successfully", nil
// }

// ---------------------------------------------------------------- designations / skills

func masterToDTO(m models.MasterEntry) models.MasterEntryResponse {
	return models.MasterEntryResponse{ID: m.ID, Name: m.Name, IsAdminApproved: m.IsAdminApproved}
}

func (s *service) SearchMaster(ctx context.Context, kind models.MasterKind, q string, limit int) ([]models.MasterEntryResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	es, err := s.repo.SearchMaster(ctx, kind, strings.TrimSpace(q), limit)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(es, masterToDTO), nil
}

// Create: a candidate typed a custom value; stored as pending (or the existing row is returned).
func (s *service) CreateMaster(ctx context.Context, kind models.MasterKind, candidateID int64, req models.MasterNameRequest) (*models.MasterEntryResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, &apperror.Error{Status: http.StatusBadRequest, Message: "name is required"}
	}
	e, err := s.repo.FindOrCreatePending(ctx, kind, name, candidateID)
	if err != nil {
		return nil, err
	}
	d := masterToDTO(*e)
	return &d, nil
}

func (s *service) ListPending(ctx context.Context, kind models.MasterKind) ([]models.MasterEntryResponse, error) {
	es, err := s.repo.ListPending(ctx, kind)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(es, masterToDTO), nil
}

func (s *service) Approve(ctx context.Context, kind models.MasterKind, id int64) error {
	return s.repo.Approve(ctx, kind, id)
}

func (s *service) Rename(ctx context.Context, kind models.MasterKind, id int64, req models.MasterNameRequest) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return &apperror.Error{Status: http.StatusBadRequest, Message: "name is required"}
	}
	return s.repo.Rename(ctx, kind, id, name)
}

func (s *service) Delete(ctx context.Context, kind models.MasterKind, id int64) error {
	return s.repo.Delete(ctx, kind, id)
}

// ---------------------------------------------------------------- companies
func companyToDTO(c models.Company) models.CompanyResponse {
	return models.CompanyResponse{CompanyID: c.CompanyID, Name: c.Name, IsRegistered: c.IsRegistered}
}

func (s *service) SearchCompany(ctx context.Context, q string) ([]models.CompanyResponse, error) {
	cs, err := s.repo.SearchCompany(ctx, strings.TrimSpace(q), 50)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(cs, companyToDTO), nil
}

func (s *service) CreateCompany(ctx context.Context, req models.CompanyRequest) (*models.CompanyResponse, error) {
	c, err := s.repo.CreateCompany(ctx, strings.TrimSpace(req.Name), req.IsRegistered)
	if err != nil {
		return nil, err
	}
	d := companyToDTO(*c)
	return &d, nil
}

func (s *service) UpdateCompany(ctx context.Context, id int64, req models.CompanyRequest) error {
	return s.repo.UpdateCompany(ctx, models.Company{CompanyID: id, Name: strings.TrimSpace(req.Name), IsRegistered: req.IsRegistered})
}

func (s *service) DeleteCompany(ctx context.Context, id int64) error {
	return s.repo.DeleteCompany(ctx, id)
}

// ---------------------------------------------------------------- locations (Country > State > City)

func locationToDTO(l models.Location) models.LocationResponse {
	return models.LocationResponse{LocationID: l.LocationID, ParentLocationID: l.ParentLocationID, Level: l.Level, Name: l.Name}
}

func (s *service) ListLocation(ctx context.Context, q models.LocationListQuery) ([]models.LocationResponse, error) {
	ls, err := s.repo.ListLocation(ctx, models.LocationFilter{
		Q: strings.TrimSpace(q.Q), Level: q.Level, ParentID: q.ParentID,
	}, 100)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(ls, locationToDTO), nil
}

func (s *service) CreateLocation(ctx context.Context, req models.LocationRequest) (*models.LocationResponse, error) {
	l, err := toLocationModel(0, req)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.CreateLocation(ctx, l)
	if err != nil {
		return nil, err
	}
	d := locationToDTO(*created)
	return &d, nil
}

func (s *service) UpdateLocation(ctx context.Context, id int64, req models.LocationRequest) error {
	l, err := toLocationModel(id, req)
	if err != nil {
		return err
	}
	return s.repo.UpdateLocation(ctx, l)
}

func (s *service) DeleteLocation(ctx context.Context, id int64) error {
	return s.repo.DeleteLocation(ctx, id)
}

func toLocationModel(id int64, r models.LocationRequest) (models.Location, error) {
	if r.Level == models.LevelCountry && r.ParentLocationID != nil {
		return models.Location{}, &apperror.Error{Status: http.StatusBadRequest, Message: "a country cannot have a parent location"}
	}
	if r.Level != models.LevelCountry && r.ParentLocationID == nil {
		return models.Location{}, &apperror.Error{Status: http.StatusBadRequest, Message: "parent_location_id is required for STATE and CITY"}
	}
	if r.ParentLocationID != nil && id != 0 && *r.ParentLocationID == id {
		return models.Location{}, &apperror.Error{Status: http.StatusBadRequest, Message: "a location cannot be its own parent"}
	}
	return models.Location{LocationID: id, ParentLocationID: r.ParentLocationID, Level: r.Level, Name: strings.TrimSpace(r.Name)}, nil
}

// ---------------------------------------------------------------- fx rates

func (s *service) GetFx(ctx context.Context, date string) ([]models.FxRateResponse, error) {
	var (
		rates []models.FxRate
		err   error
	)
	if date != "" {
		rates, err = s.repo.FxByDate(ctx, date)
	} else {
		rates, err = s.repo.FxLatest(ctx)
	}
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(rates, func(f models.FxRate) models.FxRateResponse {
		return models.FxRateResponse{RateDate: f.RateDate, CurrencyCode: f.CurrencyCode, INRConversionRate: f.INRConversionRate}
	}), nil
}

func (s *service) PutFx(ctx context.Context, req models.PutFxRatesRequest) error {
	rates := make(map[string]float64, len(req.Rates))
	for code, rate := range req.Rates {
		rates[strings.ToUpper(code)] = rate
	}
	return s.repo.UpsertFx(ctx, req.RateDate, rates)
}
