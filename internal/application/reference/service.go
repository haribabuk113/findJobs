package reference

import (
	"context"

	"findJobs/internal/domain/reference"
)

// ReferenceService defines the inbound port for reference data: taxonomies,
// locations, exchange rates and their published static JSON files.
type ReferenceService interface {
	// Manifest returns the current file per asset kind
	Manifest(ctx context.Context) (map[string]reference.ManifestEntry, error)
	// Asset returns one published file
	Asset(ctx context.Context, file string) (*reference.Asset, error)

	// ListTerms returns one page of terms for the admin review queue
	ListTerms(ctx context.Context, kind reference.Kind, status string, page, pageSize int) (*reference.TermPage, error)
	// ResolveTerms maps user-typed names to term ids, creating unknown ones as
	// pending (they go live immediately). For profile and job saves.
	ResolveTerms(ctx context.Context, kind reference.Kind, names []string, userID string) ([]reference.Term, error)
	// MergeTerms turns sourceIDs into aliases of masterID
	MergeTerms(ctx context.Context, kind reference.Kind, sourceIDs []int64, masterID int64) error
	// UpdateTerm renames and/or verifies a term
	UpdateTerm(ctx context.Context, kind reference.Kind, id int64, name, status *string) (*reference.Term, error)
	// DeleteTerm purges a term and its aliases
	DeleteTerm(ctx context.Context, kind reference.Kind, id int64) error

	// RefreshRatesAsync starts an exchange-rate refresh in the background
	// (provider retries can outlast an HTTP request); false if one is running
	RefreshRatesAsync() bool
	// RefreshRates fetches rates (3 attempts), stores them, re-normalises job
	// salaries and publishes rates.json
	RefreshRates(ctx context.Context) error
	// ImportLocations replaces the location tree and publishes it
	ImportLocations(ctx context.Context, locations []reference.Location) error
	// PublishAll republishes every asset kind (unchanged ones are skipped)
	PublishAll(ctx context.Context) error
}
