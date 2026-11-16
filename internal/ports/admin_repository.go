package ports

import (
	"context"

	"findJobs/internal/domain/admin"
)

// AuditRepository records God Mode actions. Append-only: the table's trigger
// refuses UPDATE and DELETE so the trail cannot be rewritten.
type AuditRepository interface {
	// Append inserts one audit row.
	Append(ctx context.Context, a *admin.AuditLog) error
	// List returns the most recent audit rows, newest first.
	List(ctx context.Context, limit int) ([]admin.AuditLog, error)
}

// JobReportRepository is the outbound port for job reports / moderation.
type JobReportRepository interface {
	// Create inserts a candidate or submitter's report.
	Create(ctx context.Context, r *admin.JobReport) error
	// List returns reports, filtered by status, newest first.
	List(ctx context.Context, status string, page, pageSize int) (*admin.JobReportPage, error)
	// Resolve marks a report resolved, hiding the job when reject is true.
	Resolve(ctx context.Context, id, moderatorID string, reject bool, note string) (*admin.JobReport, error)
}

// SubscriptionRepository is the outbound port for candidate premium
// subscriptions. Created and extended by platform admins at the configurable
// daily rate; a daily cron expires them and reverts the candidate tier.
type SubscriptionRepository interface {
	// Create inserts a subscription for a user.
	Create(ctx context.Context, s *admin.Subscription) error
	// Extend renews an active subscription by duration days.
	Extend(ctx context.Context, id string, durationDays int) error
	// ListByUser returns a user's subscriptions, newest first.
	ListByUser(ctx context.Context, userID string, page, pageSize int) (*admin.SubscriptionPage, error)
	// ExpireSubscriptions marks active subscriptions whose expires_at has
	// passed and returns their user ids so the caller can revert the tier.
	ExpireSubscriptions(ctx context.Context) ([]string, error)
}

// CompanyAdminPatch is the partial override a platform admin may send for a
// company (details, branding, wallet balance, tier overrides).
type CompanyAdminPatch struct {
	Name                    string `json:"name"`
	Tagline                 string `json:"tagline"`
	About                   string `json:"about"`
	Website                 string `json:"website"`
	Size                    string `json:"size"`
	Tier                    string `json:"tier"` // free | premium | none
	WalletAdjustPaise       int64  `json:"wallet_adjust_paise"`
	CreditRepostPaise       int64  `json:"credit_repost_paise"`
	CreditReviewExtendPaise int64  `json:"credit_review_extend_paise"`
}

// MemberAdminDecision is the platform admin override for one member.
type MemberAdminDecision struct {
	Role   string `json:"role"`   // admin | hr | employee | past
	Status string `json:"status"` // current | past | pending | rejected
}
