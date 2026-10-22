package companies

import (
	"context"
	"time"

	"findJobs/internal/domain/companies"
)

// CompanyService defines the inbound port for the company module.
type CompanyService interface {
	// Create publishes a company immediately (manual: no approval) with userID as its admin
	Create(ctx context.Context, userID string, in *companies.Input) (*companies.Company, error)
	// Get returns a public company page; viewerID may be ""
	Get(ctx context.Context, slug, viewerID string) (*companies.Company, error)
	// SlugAvailable checks a proposed company link
	SlugAvailable(ctx context.Context, slug string) (bool, error)
	// Search lists companies
	Search(ctx context.Context, c *companies.SearchCriteria) (*companies.SearchResult, error)
	// Update replaces the company document (admins)
	Update(ctx context.Context, userID, slug string, in *companies.Input) (*companies.Company, error)
	// Delete removes the company (admins)
	Delete(ctx context.Context, userID, slug string) error
	// SetImage replaces the logo or banner (admins); kind is "logo" or "banner"
	SetImage(ctx context.Context, userID, slug, kind, contentType string, data []byte) (*companies.Company, error)
	// File returns an uploaded image
	File(ctx context.Context, key string) ([]byte, string, error)

	// Members lists members by status (admins)
	Members(ctx context.Context, userID, slug, status string) ([]companies.Member, error)
	// DecideMember approves or rejects a pending request (admins)
	DecideMember(ctx context.Context, userID, slug, memberID string, approve bool) error
	// SetMemberRole changes a current member's role (admins)
	SetMemberRole(ctx context.Context, userID, slug, memberID, role string) error
	// Offboard moves a member to past employees (admins)
	Offboard(ctx context.Context, userID, slug, memberID string) error
	// Leave ends the caller's own membership
	Leave(ctx context.Context, userID, slug string) error
	// Dashboard returns admin overview counts (admins)
	Dashboard(ctx context.Context, userID, slug string) (*companies.Dashboard, error)

	// RequestMembership records a workplace claim. For the profile module:
	// call it when a profile experience is linked to a registered company.
	RequestMembership(ctx context.Context, userID, companyID, title string, startedOn *time.Time) error
}
