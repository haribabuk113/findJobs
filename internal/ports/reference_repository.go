package ports

import (
	"context"

	"findJobs/internal/domain/reference"
)

// TaxonomyRepository defines the outbound port for skills and designations.
type TaxonomyRepository interface {
	// List returns one page of terms, optionally filtered by status ("" = all but merged)
	List(ctx context.Context, kind reference.Kind, status string, page, pageSize int) (*reference.TermPage, error)
	// Resolve maps cleaned names to terms, creating missing ones as pending
	// (created by userID) and following merged aliases to their master.
	// Returns the terms in input order and whether any were created.
	Resolve(ctx context.Context, kind reference.Kind, names []reference.CleanedName, userID string) ([]reference.Term, bool, error)
	// Merge turns sourceIDs into aliases of masterID
	Merge(ctx context.Context, kind reference.Kind, sourceIDs []int64, masterID int64) error
	// Update renames and/or re-statuses a term
	Update(ctx context.Context, kind reference.Kind, id int64, name *reference.CleanedName, status *string) (*reference.Term, error)
	// Delete removes a term and its aliases
	Delete(ctx context.Context, kind reference.Kind, id int64) error
	// Export returns all live (non-merged) terms for the static JSON file
	Export(ctx context.Context, kind reference.Kind) ([]reference.Term, error)
}

// LocationRepository defines the outbound port for the location tree.
type LocationRepository interface {
	// ReplaceAll swaps in a full tree (import); parents must precede children
	ReplaceAll(ctx context.Context, locations []reference.Location) error
	// All returns the whole tree for the static JSON file
	All(ctx context.Context) ([]reference.Location, error)
}

// RatesRepository defines the outbound port for exchange rates.
type RatesRepository interface {
	// Replace stores new rates and recomputes the INR amounts of jobs, in one transaction
	Replace(ctx context.Context, rates reference.Rates) error
	// All returns the stored rates
	All(ctx context.Context) (reference.Rates, error)
}

// AssetRepository defines the outbound port for published static JSON files.
type AssetRepository interface {
	// Save stores the asset unless the latest of its kind has the same hash,
	// and prunes old versions. Returns whether it was stored.
	Save(ctx context.Context, asset *reference.Asset) (bool, error)
	// Get returns one asset by file name
	Get(ctx context.Context, file string) (*reference.Asset, error)
	// Manifest returns the latest file per kind
	Manifest(ctx context.Context) (map[string]reference.ManifestEntry, error)
}

// FXProvider fetches current exchange rates from an external source.
type FXProvider interface {
	// Latest returns INR per one unit of each currency
	Latest(ctx context.Context) (reference.Rates, error)
}
