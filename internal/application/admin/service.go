package admin

import (
	"context"

	"findJobs/internal/domain/admin"
	"findJobs/internal/ports"
)

// AdminService defines the platform-admin (God Mode) use cases: audit,
// job reports, subscriptions, stats and company overrides.
type AdminService interface {
	// Stats returns the platform-wide aggregates for the God Mode dashboard.
	Stats(ctx context.Context) (*ports.AdminStats, error)
	// AppendAudit records one God Mode action.
	AppendAudit(ctx context.Context, a *admin.AuditLog) error
	// ListAudit returns the most recent audit rows.
	ListAudit(ctx context.Context, limit int) ([]admin.AuditLog, error)

	// CreateReport inserts a candidate or submitter's report on a job.
	CreateReport(ctx context.Context, r *admin.JobReport) error
	// ListReports returns reports, filtered by status, paged.
	ListReports(ctx context.Context, status string, page, pageSize int) (*admin.JobReportPage, error)
	// ResolveReport marks a report resolved; reject hides the job.
	ResolveReport(ctx context.Context, id string, moderatorID string, reject bool, note string) (*admin.JobReport, error)

	// CreateSubscription grants a fixed-duration premium subscription.
	CreateSubscription(ctx context.Context, s *admin.Subscription) error
	// ExtendSubscription renews an active subscription by duration days.
	ExtendSubscription(ctx context.Context, id string, durationDays int) error
	// ListSubscriptions returns a user's subscriptions, paged.
	ListSubscriptions(ctx context.Context, userID string, page, pageSize int) (*admin.SubscriptionPage, error)
	// ExpireSubscriptions marks expired subscriptions and returns user ids.
	ExpireSubscriptions(ctx context.Context) ([]string, error)

	// ListCompanies returns companies for the God Mode dashboard, filtered, paged.
	ListCompanies(ctx context.Context, in *CompanyListInput) (*admin.CompanyPage, error)
	// GetCompany returns the full admin view of one company.
	GetCompany(ctx context.Context, slug string) (*admin.CompanyAdmin, error)
	// PatchCompany overrides company details, branding, wallet and tier for a
	// platform admin (God Mode), bypassing the company-admin checks.
	PatchCompany(ctx context.Context, slug, adminID string, in *ports.CompanyAdminPatch) (*admin.CompanyAdmin, error)
	// DeleteCompany hard-deletes a company (soft delete is for its own admins).
	DeleteCompany(ctx context.Context, slug, adminID string) error
	// SetMemberDecision applies a platform admin override for one member.
	SetMemberDecision(ctx context.Context, slug, adminID, memberID string, in *ports.MemberAdminDecision) error
}

// CompanyListInput is the query for GET /admin/companies.
type CompanyListInput struct {
	Query    string
	Page     int
	PageSize int
}