package ports

import (
	"context"
	"time"

	"findJobs/internal/domain/companies"
)

// CompanyRepository defines the outbound port for companies and their members.
// Every admin-only method takes the acting user and checks, in the same
// transaction, that they are a current admin of that company.
type CompanyRepository interface {
	// LocationCountries returns the country code of each existing location id
	LocationCountries(ctx context.Context, ids []string) (map[string]string, error)
	// Create stores the company and makes userID its first admin
	Create(ctx context.Context, in *companies.Input, hqCountry, userID string) (*companies.Company, error)
	// GetBySlug returns a live company; viewerID ("" if anonymous) fills ViewerRole
	GetBySlug(ctx context.Context, slug, viewerID string) (*companies.Company, error)
	// SlugTaken reports whether any company (including deleted ones) uses slug
	SlugTaken(ctx context.Context, slug string) (bool, error)
	// Search returns one page of live companies
	Search(ctx context.Context, c *companies.SearchCriteria) (*companies.SearchResult, error)
	// Update replaces the company document (admin only)
	Update(ctx context.Context, slug, userID string, in *companies.Input, hqCountry string) (*companies.Company, error)
	// Delete soft-deletes the company, hides its jobs and ends memberships (admin only)
	Delete(ctx context.Context, slug, userID string) error
	// SetImage stores a logo or banner key (admin only) and returns the replaced key
	SetImage(ctx context.Context, slug, userID, kind, key string) (string, error)

	// Members lists members with the given status (admin only)
	Members(ctx context.Context, slug, userID, status string) ([]companies.Member, error)
	// DecideMember approves (current) or rejects a pending request (admin only)
	DecideMember(ctx context.Context, slug, adminID, memberID string, approve bool) error
	// SetMemberRole changes a current member's role, keeping at least one admin (admin only)
	SetMemberRole(ctx context.Context, slug, adminID, memberID, role string) error
	// Offboard moves a current member to past, keeping at least one admin (admin only)
	Offboard(ctx context.Context, slug, adminID, memberID string) error
	// Leave ends userID's own membership, keeping at least one admin
	Leave(ctx context.Context, slug, userID string) error
	// RequestMembership records a workplace claim from a profile as pending
	RequestMembership(ctx context.Context, companyID, userID, title string, startedOn *time.Time) error
	// Dashboard returns the admin overview counts (admin only)
	Dashboard(ctx context.Context, slug, userID string) (*companies.Dashboard, error)
}

// ObjectStore defines the outbound port for uploaded files (local disk in
// dev, S3-compatible storage in production).
type ObjectStore interface {
	// Put stores data under key
	Put(ctx context.Context, key, contentType string, data []byte) error
	// Get returns a stored file and its content type
	Get(ctx context.Context, key string) ([]byte, string, error)
	// Delete removes a file; a missing key is not an error
	Delete(ctx context.Context, key string) error
}
