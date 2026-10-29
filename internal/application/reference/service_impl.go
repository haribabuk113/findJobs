package reference

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"findJobs/internal/domain/reference"
	"findJobs/internal/pkg/background"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

const (
	maxResolveNames = 50
	maxMergeSources = 100
	maxPageSize     = 100
	// publishDelay batches the static-file rebuild after custom terms are
	// created, so a burst of profile/job saves triggers one publish.
	publishDelay = 5 * time.Second
	fxAttempts   = 3
)

// assetFile matches what publish names files; anything else is a 404
// without touching the database.
var assetFile = regexp.MustCompile(`^(skills|designations|locations|rates)-\d{8}T\d{6}Z-[0-9a-f]{8}\.json$`)

type service struct {
	taxonomy  ports.TaxonomyRepository
	locations ports.LocationRepository
	rates     ports.RatesRepository
	assets    ports.AssetRepository
	fx        ports.FXProvider
	bg        *background.Group // refreshes and debounced publishes; stopped on shutdown

	mu         sync.Mutex
	scheduled  map[string]bool
	refreshing atomic.Bool
	retryWait  time.Duration
}

// NewReferenceService creates the reference-data service using dependency injection.
func NewReferenceService(bg *background.Group, taxonomy ports.TaxonomyRepository, locations ports.LocationRepository,
	rates ports.RatesRepository, assets ports.AssetRepository, fx ports.FXProvider) ReferenceService {
	return &service{bg: bg, taxonomy: taxonomy, locations: locations, rates: rates, assets: assets, fx: fx,
		scheduled: map[string]bool{}, retryWait: 5 * time.Second}
}

func (s *service) Manifest(ctx context.Context) (map[string]reference.ManifestEntry, error) {
	return s.assets.Manifest(ctx)
}

func (s *service) Asset(ctx context.Context, file string) (*reference.Asset, error) {
	if !assetFile.MatchString(file) {
		return nil, &reference.NotFoundError{Message: "asset not found"}
	}
	return s.assets.Get(ctx, file)
}

func (s *service) ListTerms(ctx context.Context, kind reference.Kind, status string, page, pageSize int) (*reference.TermPage, error) {
	switch status {
	case "", reference.StatusPending, reference.StatusVerified, reference.StatusMerged:
	default:
		return nil, &reference.ValidationError{Field: "status", Message: "status must be pending, verified or merged"}
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 50
	}
	if page < 1 || page > 1000 || pageSize < 1 || pageSize > maxPageSize {
		return nil, &reference.ValidationError{Field: "page", Message: "page must be 1-1000 and page_size 1-100"}
	}
	return s.taxonomy.List(ctx, kind, status, page, pageSize)
}

func (s *service) ResolveTerms(ctx context.Context, kind reference.Kind, names []string, userID string) ([]reference.Term, error) {
	if len(names) > maxResolveNames {
		return nil, &reference.ValidationError{Field: "names", Message: fmt.Sprintf("at most %d names per request", maxResolveNames)}
	}
	cleaned := make([]reference.CleanedName, 0, len(names))
	seen := map[string]bool{}
	for _, raw := range names {
		name, err := reference.CleanName(raw)
		if err != nil {
			return nil, err
		}
		if !seen[name.Normalized] {
			seen[name.Normalized] = true
			cleaned = append(cleaned, name)
		}
	}
	if len(cleaned) == 0 {
		return []reference.Term{}, nil
	}
	terms, created, err := s.taxonomy.Resolve(ctx, kind, cleaned, userID)
	if err != nil {
		return nil, err
	}
	if created {
		s.publishSoon(kind.AssetKind()) // manual: new terms reach every client's autocomplete
	}
	return terms, nil
}

func (s *service) MergeTerms(ctx context.Context, kind reference.Kind, sourceIDs []int64, masterID int64) error {
	slices.Sort(sourceIDs)
	sourceIDs = slices.Compact(sourceIDs)
	if len(sourceIDs) == 0 || len(sourceIDs) > maxMergeSources || masterID <= 0 || sourceIDs[0] <= 0 {
		return &reference.ValidationError{Field: "source_ids", Message: fmt.Sprintf("give 1-%d source ids and a master id", maxMergeSources)}
	}
	if slices.Contains(sourceIDs, masterID) {
		return &reference.ValidationError{Field: "master_id", Message: "the master can't also be a source"}
	}
	if err := s.taxonomy.Merge(ctx, kind, sourceIDs, masterID); err != nil {
		return err
	}
	return s.publish(ctx, kind.AssetKind())
}

func (s *service) UpdateTerm(ctx context.Context, kind reference.Kind, id int64, name, status *string) (*reference.Term, error) {
	if name == nil && status == nil {
		return nil, &reference.ValidationError{Field: "name", Message: "give a name and/or a status"}
	}
	if status != nil && *status != reference.StatusVerified && *status != reference.StatusPending {
		return nil, &reference.ValidationError{Field: "status", Message: "status must be verified or pending (use merge for aliases)"}
	}
	var cleaned *reference.CleanedName
	if name != nil {
		c, err := reference.CleanName(*name)
		if err != nil {
			return nil, err
		}
		cleaned = &c
	}
	term, err := s.taxonomy.Update(ctx, kind, id, cleaned, status)
	if err != nil {
		return nil, err
	}
	return term, s.publish(ctx, kind.AssetKind())
}

func (s *service) DeleteTerm(ctx context.Context, kind reference.Kind, id int64) error {
	if err := s.taxonomy.Delete(ctx, kind, id); err != nil {
		return err
	}
	return s.publish(ctx, kind.AssetKind())
}

func (s *service) RefreshRatesAsync() bool {
	if !s.refreshing.CompareAndSwap(false, true) {
		return false
	}
	started := s.bg.Go(func(ctx context.Context) {
		defer s.refreshing.Store(false)
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := s.RefreshRates(ctx); err != nil {
			s.logFailure(err, "exchange-rate refresh failed; keeping the last published rates")
		}
	})
	if !started { // shutting down
		s.refreshing.Store(false)
	}
	return started
}

// logFailure logs background work that failed, except a cancellation by
// shutdown, which is expected: the transaction rolled back and the work is
// redone later (the next refresh, or PublishAll at the next start).
func (s *service) logFailure(err error, msg string) {
	l := logger.Get()
	if s.bg.Stopping() {
		l.Info().Err(err).Msg("stopped by shutdown: " + msg)
		return
	}
	l.Error().Err(err).Msg(msg)
}

// RefreshRates follows manual "Salary": up to 3 attempts; on failure nothing
// changes, so clients keep the last valid rates file.
// ponytail: failures are only logged; the manual's admin alert needs a
// Notifier (email) port, add it with the first notification feature.
func (s *service) RefreshRates(ctx context.Context) error {
	var rates reference.Rates
	var err error
	for attempt := 1; attempt <= fxAttempts; attempt++ {
		if rates, err = s.fx.Latest(ctx); err == nil {
			break
		}
		if attempt < fxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * s.retryWait):
			}
		}
	}
	if err != nil {
		return fmt.Errorf("fx provider failed %d times: %w", fxAttempts, err)
	}
	if err := s.rates.Replace(ctx, rates); err != nil {
		return err
	}
	return s.publish(ctx, reference.AssetRates)
}

func (s *service) ImportLocations(ctx context.Context, locations []reference.Location) error {
	if err := s.locations.ReplaceAll(ctx, locations); err != nil {
		return err
	}
	return s.publish(ctx, reference.AssetLocations)
}

func (s *service) PublishAll(ctx context.Context) error {
	for _, kind := range []string{reference.AssetSkills, reference.AssetDesignations, reference.AssetLocations, reference.AssetRates} {
		if err := s.publish(ctx, kind); err != nil {
			return fmt.Errorf("publish %s: %w", kind, err)
		}
	}
	return nil
}

// publishSoon schedules one publish of kind after publishDelay; calls in
// between collapse into it. A publish still pending at shutdown is dropped;
// PublishAll at the next start catches up (unchanged exports are skipped).
func (s *service) publishSoon(kind string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scheduled[kind] {
		return
	}
	s.scheduled[kind] = true
	time.AfterFunc(publishDelay, func() {
		s.mu.Lock()
		delete(s.scheduled, kind)
		s.mu.Unlock()
		s.bg.Go(func(ctx context.Context) {
			if err := s.publish(ctx, kind); err != nil {
				s.logFailure(err, "asset publish failed: "+kind)
			}
		})
	})
}

// publish exports one kind to a timestamped JSON file (manual: strict UTC
// names, so browsers never serve a stale cached copy). An unchanged export
// keeps the current file.
func (s *service) publish(ctx context.Context, kind string) error {
	var payload any
	switch kind {
	case reference.AssetSkills, reference.AssetDesignations:
		k := reference.Skill
		if kind == reference.AssetDesignations {
			k = reference.Designation
		}
		terms, err := s.taxonomy.Export(ctx, k)
		if err != nil {
			return err
		}
		items := make([]termItem, len(terms))
		for i, t := range terms {
			items[i] = termItem{ID: t.ID, Name: t.Name, Verified: t.Status == reference.StatusVerified}
		}
		payload = items
	case reference.AssetLocations:
		locations, err := s.locations.All(ctx)
		if err != nil {
			return err
		}
		payload = locations
	case reference.AssetRates:
		rates, err := s.rates.All(ctx)
		if err != nil {
			return err
		}
		if len(rates) == 0 {
			return nil // nothing fetched yet
		}
		payload = ratesFile{Base: "INR", Rates: rates}
	default:
		return fmt.Errorf("unknown asset kind %q", kind)
	}

	content, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(content)
	hash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	_, err = s.assets.Save(ctx, &reference.Asset{
		File:      fmt.Sprintf("%s-%s-%s.json", kind, now.Format("20060102T150405Z"), hash[:8]),
		Kind:      kind,
		Content:   content,
		SHA256:    hash,
		CreatedAt: now,
	})
	return err
}

type termItem struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Verified bool   `json:"verified"`
}

// ratesFile: INR per one unit of each currency. Client conversion (manual
// "Salary"): display = inr_amount / rates[c]; filter INR = amount × rates[c].
type ratesFile struct {
	Base  string          `json:"base"`
	Rates reference.Rates `json:"rates"`
}
