package admin

import (
	"context"

	"findJobs/internal/domain/admin"
	"findJobs/internal/ports"
)

// service implements the platform-admin (God Mode) use cases by delegating
// to the outbound repository ports.
type service struct {
	auditRepo   ports.AuditRepository
	reportRepo  ports.JobReportRepository
	subRepo     ports.SubscriptionRepository
	companyRepo ports.CompanyRepository
	statsRepo   ports.StatsRepository
}

// NewAdminService constructs an AdminService backed by the given repository ports.
func NewAdminService(
	auditRepo ports.AuditRepository,
	reportRepo ports.JobReportRepository,
	subRepo ports.SubscriptionRepository,
	companyRepo ports.CompanyRepository,
	statsRepo ports.StatsRepository,
) AdminService {
	return &service{
		auditRepo:   auditRepo,
		reportRepo:  reportRepo,
		subRepo:     subRepo,
		companyRepo: companyRepo,
		statsRepo:   statsRepo,
	}
}

// Stats returns the platform-wide aggregates for the God Mode dashboard.
func (s *service) Stats(ctx context.Context) (*ports.AdminStats, error) {
	return s.statsRepo.PlatformStats(ctx)
}

// AppendAudit records one God Mode action.
func (s *service) AppendAudit(ctx context.Context, a *admin.AuditLog) error {
	return s.auditRepo.Append(ctx, a)
}

// ListAudit returns the most recent audit rows.
func (s *service) ListAudit(ctx context.Context, limit int) ([]admin.AuditLog, error) {
	return s.auditRepo.List(ctx, limit)
}

// CreateReport inserts a candidate or submitter's report on a job.
func (s *service) CreateReport(ctx context.Context, r *admin.JobReport) error {
	return s.reportRepo.Create(ctx, r)
}

// ListReports returns reports, filtered by status, paged.
func (s *service) ListReports(ctx context.Context, status string, page, pageSize int) (*admin.JobReportPage, error) {
	return s.reportRepo.List(ctx, status, page, pageSize)
}

// ResolveReport marks a report resolved; reject hides the job.
func (s *service) ResolveReport(ctx context.Context, id string, moderatorID string, reject bool, note string) (*admin.JobReport, error) {
	return s.reportRepo.Resolve(ctx, id, moderatorID, reject, note)
}

// CreateSubscription grants a fixed-duration premium subscription.
func (s *service) CreateSubscription(ctx context.Context, sSub *admin.Subscription) error {
	return s.subRepo.Create(ctx, sSub)
}

// ExtendSubscription renews an active subscription by duration days.
func (s *service) ExtendSubscription(ctx context.Context, id string, durationDays int) error {
	return s.subRepo.Extend(ctx, id, durationDays)
}

// ListSubscriptions returns a user's subscriptions, paged.
func (s *service) ListSubscriptions(ctx context.Context, userID string, page, pageSize int) (*admin.SubscriptionPage, error) {
	return s.subRepo.ListByUser(ctx, userID, page, pageSize)
}

// ExpireSubscriptions marks expired subscriptions and returns user ids.
func (s *service) ExpireSubscriptions(ctx context.Context) ([]string, error) {
	return s.subRepo.ExpireSubscriptions(ctx)
}

// ListCompanies returns companies for the God Mode dashboard, filtered, paged.
func (s *service) ListCompanies(ctx context.Context, in *CompanyListInput) (*admin.CompanyPage, error) {
	return s.companyRepo.AdminListCompanies(ctx, in.Query, in.Page, in.PageSize)
}

// GetCompany returns the full admin view of one company.
func (s *service) GetCompany(ctx context.Context, slug string) (*admin.CompanyAdmin, error) {
	return s.companyRepo.AdminGetCompany(ctx, slug)
}

// PatchCompany overrides company details, branding, wallet and tier for a
// platform admin (God Mode), bypassing the company-admin checks.
func (s *service) PatchCompany(ctx context.Context, slug, adminID string, in *ports.CompanyAdminPatch) (*admin.CompanyAdmin, error) {
	return s.companyRepo.AdminPatchCompany(ctx, slug, adminID, in)
}

// DeleteCompany hard-deletes a company (soft delete is for its own admins).
func (s *service) DeleteCompany(ctx context.Context, slug, adminID string) error {
	return s.companyRepo.AdminDeleteCompany(ctx, slug, adminID)
}

// SetMemberDecision applies a platform admin override for one member.
func (s *service) SetMemberDecision(ctx context.Context, slug, adminID, memberID string, in *ports.MemberAdminDecision) error {
	return s.companyRepo.AdminSetMemberDecision(ctx, slug, adminID, memberID, in)
}