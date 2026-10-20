package companies

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/companies"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

type service struct {
	repo  ports.CompanyRepository
	store ports.ObjectStore
	now   func() time.Time
}

// NewCompanyService creates the company service using dependency injection.
func NewCompanyService(repo ports.CompanyRepository, store ports.ObjectStore) CompanyService {
	return &service{repo: repo, store: store, now: time.Now}
}

func (s *service) Create(ctx context.Context, userID string, in *companies.Input) (*companies.Company, error) {
	country, err := s.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, in, country, userID)
}

func (s *service) Get(ctx context.Context, slug, viewerID string) (*companies.Company, error) {
	if companies.ValidateSlug(slug) != nil {
		return nil, companies.ErrNotFound
	}
	return s.repo.GetBySlug(ctx, slug, viewerID)
}

func (s *service) SlugAvailable(ctx context.Context, slug string) (bool, error) {
	if err := companies.ValidateSlug(slug); err != nil {
		return false, err
	}
	taken, err := s.repo.SlugTaken(ctx, slug)
	return !taken, err
}

func (s *service) Search(ctx context.Context, c *companies.SearchCriteria) (*companies.SearchResult, error) {
	if err := c.Normalize(); err != nil {
		return nil, err
	}
	return s.repo.Search(ctx, c)
}

func (s *service) Update(ctx context.Context, userID, slug string, in *companies.Input) (*companies.Company, error) {
	country, err := s.prepare(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, slug, userID, in, country)
}

func (s *service) Delete(ctx context.Context, userID, slug string) error {
	return s.repo.Delete(ctx, slug, userID)
}

func (s *service) SetImage(ctx context.Context, userID, slug, kind, contentType string, data []byte) (*companies.Company, error) {
	limit := companies.MaxLogoBytes
	if kind == "banner" {
		limit = companies.MaxBannerBytes
	}
	ext, err := companies.CheckImage(data, contentType, limit)
	if err != nil {
		return nil, err
	}
	key := "img/" + uuid.NewString() + "." + ext
	if err := s.store.Put(ctx, key, contentType, data); err != nil {
		return nil, err
	}
	old, err := s.repo.SetImage(ctx, slug, userID, kind, key)
	if err != nil {
		s.store.Delete(context.WithoutCancel(ctx), key) // not linked: don't leave an orphan
		return nil, err
	}
	if old != "" {
		if err := s.store.Delete(ctx, old); err != nil {
			l := logger.Get()
			l.Warn().Err(err).Str("key", old).Msg("could not delete replaced company image")
		}
	}
	return s.repo.GetBySlug(ctx, slug, userID)
}

func (s *service) File(ctx context.Context, key string) ([]byte, string, error) {
	return s.store.Get(ctx, key)
}

func (s *service) Members(ctx context.Context, userID, slug, status string) ([]companies.Member, error) {
	if !slices.Contains([]string{companies.StatusPending, companies.StatusCurrent, companies.StatusPast, companies.StatusRejected}, status) {
		return nil, &companies.ValidationError{Field: "status", Message: "status must be pending, current, past or rejected"}
	}
	return s.repo.Members(ctx, slug, userID, status)
}

func (s *service) DecideMember(ctx context.Context, userID, slug, memberID string, approve bool) error {
	if uuid.Validate(memberID) != nil {
		return &companies.NotFoundError{Message: "member not found"}
	}
	return s.repo.DecideMember(ctx, slug, userID, memberID, approve)
}

func (s *service) SetMemberRole(ctx context.Context, userID, slug, memberID, role string) error {
	if !slices.Contains([]string{companies.RoleAdmin, companies.RoleHR, companies.RoleEmployee}, role) {
		return &companies.ValidationError{Field: "role", Message: "role must be admin, hr or employee"}
	}
	if uuid.Validate(memberID) != nil {
		return &companies.NotFoundError{Message: "member not found"}
	}
	return s.repo.SetMemberRole(ctx, slug, userID, memberID, role)
}

func (s *service) Offboard(ctx context.Context, userID, slug, memberID string) error {
	if uuid.Validate(memberID) != nil {
		return &companies.NotFoundError{Message: "member not found"}
	}
	return s.repo.Offboard(ctx, slug, userID, memberID)
}

func (s *service) Leave(ctx context.Context, userID, slug string) error {
	return s.repo.Leave(ctx, slug, userID)
}

func (s *service) Dashboard(ctx context.Context, userID, slug string) (*companies.Dashboard, error) {
	return s.repo.Dashboard(ctx, slug, userID)
}

func (s *service) RequestMembership(ctx context.Context, userID, companyID, title string, startedOn *time.Time) error {
	return s.repo.RequestMembership(ctx, companyID, userID, title, startedOn)
}

// prepare validates the input and resolves the headquarters' country; every
// location id must exist in the published tree.
func (s *service) prepare(ctx context.Context, in *companies.Input) (string, error) {
	if err := in.Normalize(s.now()); err != nil {
		return "", err
	}
	countries, err := s.repo.LocationCountries(ctx, append([]string{in.HQLocationID}, in.BranchLocationIDs...))
	if err != nil {
		return "", err
	}
	country, ok := countries[in.HQLocationID]
	if !ok {
		return "", &companies.ValidationError{Field: "hq_location_id", Message: "unknown headquarters location"}
	}
	for _, id := range in.BranchLocationIDs {
		if _, ok := countries[id]; !ok {
			return "", &companies.ValidationError{Field: "branch_location_ids", Message: "unknown branch location"}
		}
	}
	return country, nil
}
