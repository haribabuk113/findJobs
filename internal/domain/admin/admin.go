// Package admin holds the platform-admin (God Mode) domain types: audit
// trail, job reports and candidate subscriptions. These are the admin
// portal's own data, separate from the modules that own the records.
package admin

import (
	"time"
)

// ActorType distinguishes who performed a God Mode action.
const (
	ActorUser    = "user"
	ActorCompany = "company"
	ActorAdmin   = "admin"
)

// TargetType names the kind of record an audit row points at.
const (
	TargetCompany      = "company"
	TargetJob          = "job"
	TargetUser         = "user"
	TargetTerm         = "term"
	TargetReport       = "report"
	TargetSubscription = "subscription"
	TargetWallet       = "wallet"
)

// AuditLog is one God Mode action, recorded after the fact (manual §4.9).
// Append-only: the table's trigger refuses UPDATE and DELETE.
type AuditLog struct {
	ID         int64          `json:"id"`
	ActorID    string         `json:"actor_id"`
	ActorType  string         `json:"actor_type"`
	Action     string         `json:"action"`
	TargetID   string         `json:"target_id"`
	TargetType string         `json:"target_type"`
	Changes    map[string]any `json:"changes"`
	CreatedAt  time.Time      `json:"created_at"`
}

// ReportStatus is the moderation lifecycle of a job report (manual §4.5).
const (
	ReportNew          = "new"
	ReportAcknowledged = "acknowledged"
	ReportResolved     = "resolved"
)

// JobReport is a candidate or submitter flagging a job (manual §4.5).
type JobReport struct {
	ID             string     `json:"id"`
	JobID          string     `json:"job_id"`
	ReporterID     string     `json:"reporter_id"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	ModerationNote string     `json:"moderation_note"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}

// SubscriptionStatus is the lifecycle of a candidate premium subscription.
const (
	SubActive  = "active"
	SubExpired = "expired"
)

// Subscription is a candidate's premium subscription (manual §4.4).
type Subscription struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	DurationDays int       `json:"duration_days"`
	StartedAt    time.Time `json:"started_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       string    `json:"status"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

// Validate checks the fields an admin submitted.
func (s *Subscription) Validate() error {
	if s.UserID == "" {
		return &ValidationError{Field: "user_id", Message: "user_id is required"}
	}
	if s.DurationDays < 1 || s.DurationDays > 365 {
		return &ValidationError{Field: "duration_days", Message: "duration_days must be between 1 and 365"}
	}
	if s.Status != "" && s.Status != SubActive && s.Status != SubExpired {
		return &ValidationError{Field: "status", Message: "status must be active or expired"}
	}
	return nil
}

// ValidationError is a rejected field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// IsValidation reports whether err is a ValidationError.
// NotFoundError: no such report or subscription.
type NotFoundError struct{ Message string }

func (e *NotFoundError) Error() string { return e.Message }

// ConflictError: an invalid state transition.
type ConflictError struct{ Message string }

func (e *ConflictError) Error() string { return e.Message }

// IsNotFound reports whether err is a NotFoundError.
func IsNotFound(err error) bool { _, ok := err.(*NotFoundError); return ok }

// IsConflict reports whether err is a ConflictError.
func IsConflict(err error) bool { _, ok := err.(*ConflictError); return ok }

func IsValidation(err error) bool { _, ok := err.(*ValidationError); return ok }

// CompanyAdmin is the full God Mode view of one company: details, branding,
// wallet balance and tier, plus its members (manual §4.2).
type CompanyAdmin struct {
	ID                      string        `json:"id"`
	Slug                    string        `json:"slug"`
	Name                    string        `json:"name"`
	Tagline                 string        `json:"tagline"`
	About                   string        `json:"about"`
	Website                 string        `json:"website"`
	Size                    string        `json:"size"`
	Tier                    string        `json:"tier"` // free | premium | none
	WalletBalancePaise      int64         `json:"wallet_balance_paise"`
	CreditRepostPaise       int64         `json:"credit_repost_paise"`
	CreditReviewExtendPaise int64         `json:"credit_review_extend_paise"`
	Members                 []MemberAdmin `json:"members"`
	CreatedAt               time.Time     `json:"created_at"`
}

// MemberAdmin is one member as seen by a platform admin (God Mode).
type MemberAdmin struct {
	UserID     string `json:"user_id"`
	Role       string `json:"role"`   // admin | hr | employee
	Status     string `json:"status"` // current | past | pending | rejected
	LimitPaise int64  `json:"limit_paise"`
	SpentPaise int64  `json:"spent_paise"`
	SpentMonth string `json:"spent_month"`
}

// CompanyPage is one page of companies for the God Mode dashboard.
type CompanyPage struct {
	Companies []CompanyAdmin
	Page      int
	PageSize  int
	Total     int
}

// JobReportPage is one page of job reports.
type JobReportPage struct {
	Reports  []JobReport
	Page     int
	PageSize int
	Total    int
}

// SubscriptionPage is one page of subscriptions.
type SubscriptionPage struct {
	Subscriptions []Subscription
	Page          int
	PageSize      int
	Total         int
}
