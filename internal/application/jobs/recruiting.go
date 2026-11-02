package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"findJobs/internal/domain/jobs"
	"findJobs/internal/domain/payments"
	"findJobs/internal/domain/reference"
	"findJobs/internal/ports"
)

// JobPostPayer collects out-of-pocket job post fees through the gateway (the payment service).
type JobPostPayer interface {
	Configured() bool
	JobPostCheckout(ctx context.Context, userID, jobID string, amountPaise int64) (*payments.Checkout, error)
}

// TermResolver turns typed names into taxonomy ids (the reference service).
type TermResolver interface {
	ResolveTerms(ctx context.Context, kind reference.Kind, names []string, userID string) ([]reference.Term, error)
}

// RecruitingService defines the inbound port for posting and managing jobs.
type RecruitingService interface {
	PosterCompanies(ctx context.Context, userID string) ([]jobs.PosterCompany, error)
	// Post publishes a job. A paid post with a fee needs idempotencyKey (the
	// request's Idempotency-Key): a retry with the same key returns the first
	// job. Company credits are charged at once; out of pocket returns the job
	// pending with a Checkout, and the payment webhook publishes it.
	Post(ctx context.Context, userID, idempotencyKey string, in *jobs.PostInput) (*jobs.Detail, error)
	// Checkout starts a new payment for the user's pending job (e.g. after an abandoned checkout).
	Checkout(ctx context.Context, userID, id string) (*jobs.Detail, error)
	Edit(ctx context.Context, userID, id string, in *jobs.PostInput) (*jobs.Detail, error)
	Close(ctx context.Context, userID, id string) error
	Repost(ctx context.Context, userID, id string) error
	Mine(ctx context.Context, userID, state string) ([]jobs.Posted, error)

	// Detail is the job page by id; Shared by share hash (free jobs' only way in).
	Detail(ctx context.Context, id, viewerID string) (*jobs.Detail, error)
	Shared(ctx context.Context, hash, viewerID string) (*jobs.Detail, error)
	Outbound(ctx context.Context, id string) error
	Recruiter(ctx context.Context, id string) (*jobs.Person, int, error)

	Drafts(ctx context.Context, userID string) ([]jobs.Draft, error)
	SaveDraft(ctx context.Context, userID, id string, form map[string]any) (*jobs.Draft, error)
	DeleteDraft(ctx context.Context, userID, id string) error
}

type recruiting struct {
	repo     ports.RecruitingRepository
	terms    TermResolver
	settings ports.SettingsRepository
	payer    JobPostPayer
}

// NewRecruitingService creates the recruiting service using dependency injection.
func NewRecruitingService(repo ports.RecruitingRepository, terms TermResolver, settings ports.SettingsRepository,
	payer JobPostPayer) RecruitingService {
	return &recruiting{repo: repo, terms: terms, settings: settings, payer: payer}
}

var shareHash = regexp.MustCompile(`^[0-9a-f]{32}$`)

// idempotencyKey is a client-generated key, e.g. a uuid.
var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,100}$`)

const maxDraftBytes = 64 << 10

func (s *recruiting) PosterCompanies(ctx context.Context, userID string) ([]jobs.PosterCompany, error) {
	list, err := s.repo.PosterCompanies(ctx, userID)
	if list == nil {
		list = []jobs.PosterCompany{}
	}
	return list, err
}

func (s *recruiting) Post(ctx context.Context, userID, key string, in *jobs.PostInput) (*jobs.Detail, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	if key != "" && !idempotencyKey.MatchString(key) {
		return nil, &jobs.ValidationError{Field: "Idempotency-Key", Message: "Idempotency-Key must be 8 to 100 letters, digits, - or _"}
	}
	var charge *jobs.Charge
	if in.Tier == jobs.TierPaid {
		settings, err := s.settings.Get(ctx)
		if err != nil {
			return nil, err
		}
		// A fee of 0 (JOB_POST_FEE_PAISE, default ₹2) publishes paid posts free.
		if settings.JobPostFeePaise > 0 {
			if key == "" {
				return nil, &jobs.ValidationError{Field: "Idempotency-Key", Message: "a paid post needs an Idempotency-Key header"}
			}
			if in.PaymentMethod == jobs.PayOutOfPocket && !s.payer.Configured() {
				return nil, payments.ErrNotConfigured
			}
			charge = &jobs.Charge{Method: in.PaymentMethod, FeePaise: settings.JobPostFeePaise}
		}
	}
	if key != "" {
		// A retry of a post that went through: return that job, charge nothing.
		if id, err := s.repo.PostByKey(ctx, userID, key); err != nil || id != "" {
			if err != nil {
				return nil, err
			}
			return s.repo.Detail(ctx, id, false, userID)
		}
	}
	rec, err := s.record(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	rec.ID = uuid.NewString()
	rec.ShareHash = randomHash()
	rec.IdempotencyKey = key
	if err := s.repo.Create(ctx, rec, charge); err != nil {
		return nil, err
	}
	d, err := s.repo.Detail(ctx, rec.ID, false, userID)
	if err != nil || charge == nil || charge.Method != jobs.PayOutOfPocket {
		return d, err
	}
	// If this fails the job stays pending; Checkout retries the payment.
	d.Checkout, err = s.payer.JobPostCheckout(ctx, userID, rec.ID, charge.FeePaise)
	return d, err
}

func (s *recruiting) Checkout(ctx context.Context, userID, id string) (*jobs.Detail, error) {
	if uuid.Validate(id) != nil {
		return nil, jobs.ErrJobNotFound
	}
	pending, err := s.repo.Pending(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !pending {
		return nil, jobs.ErrAlreadyPaid
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, err
	}
	var checkout *payments.Checkout
	if settings.JobPostFeePaise == 0 {
		// Posting became free while this job waited: publish it.
		if err := s.repo.PublishPending(ctx, id, userID); err != nil {
			return nil, err
		}
	} else if checkout, err = s.payer.JobPostCheckout(ctx, userID, id, settings.JobPostFeePaise); err != nil {
		return nil, err
	}
	d, err := s.repo.Detail(ctx, id, false, userID)
	if d != nil {
		d.Checkout = checkout
	}
	return d, err
}

func (s *recruiting) Edit(ctx context.Context, userID, id string, in *jobs.PostInput) (*jobs.Detail, error) {
	if uuid.Validate(id) != nil && !legacyID.MatchString(id) {
		return nil, jobs.ErrJobNotFound
	}
	tier, slug, _, err := s.repo.Editable(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	// Tier, company and payment are fixed once published; only the content changes.
	in.Tier, in.CompanySlug, in.PaymentMethod = jobs.TierFree, "", ""
	if tier == "Premium" {
		in.Tier, in.PaymentMethod = jobs.TierPaid, jobs.PayOutOfPocket
		if slug != nil {
			in.CompanySlug = *slug
		}
	}
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	rec, err := s.record(ctx, userID, in)
	if err != nil {
		return nil, err
	}
	rec.ID = id
	if err := s.repo.Update(ctx, rec); err != nil {
		return nil, err
	}
	return s.repo.Detail(ctx, id, false, userID)
}

// record checks references and resolves terms for a normalized form.
func (s *recruiting) record(ctx context.Context, userID string, in *jobs.PostInput) (*jobs.Record, error) {
	rec := &jobs.Record{PostInput: *in, RecruiterID: userID, DBTier: "Free"}
	if in.Tier == jobs.TierPaid {
		rec.DBTier, rec.Listed = "Premium", true
	}
	if in.CompanySlug != "" {
		companyID, err := s.repo.MemberCompanyID(ctx, in.CompanySlug, userID)
		if err != nil {
			return nil, err
		}
		rec.CompanyID = &companyID
	}
	missing, err := s.repo.MissingLocations(ctx, in.LocationIDs)
	if err != nil {
		return nil, err
	}
	if len(missing) > 0 {
		return nil, &jobs.ValidationError{Field: "location_ids", Message: "unknown location " + missing[0]}
	}
	known, err := s.repo.CurrencyKnown(ctx, in.Currency)
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, &jobs.ValidationError{Field: "currency", Message: "unsupported currency " + in.Currency}
	}
	// Manual: typed designations and skills go live immediately as pending terms.
	designation, err := s.terms.ResolveTerms(ctx, reference.Designation, []string{in.Designation}, userID)
	if err != nil {
		return nil, asJobValidation("designation", err)
	}
	rec.DesignationID = designation[0].ID
	skills, err := s.terms.ResolveTerms(ctx, reference.Skill, in.Skills, userID)
	if err != nil {
		return nil, asJobValidation("skills", err)
	}
	seen := map[int64]bool{}
	for _, t := range skills {
		if !seen[t.ID] { // two typed variants can resolve to one master
			seen[t.ID] = true
			rec.SkillIDs = append(rec.SkillIDs, t.ID)
		}
	}
	return rec, nil
}

func (s *recruiting) Close(ctx context.Context, userID, id string) error {
	return s.repo.Close(ctx, id, userID)
}

func (s *recruiting) Repost(ctx context.Context, userID, id string) error {
	return s.repo.Repost(ctx, id, userID)
}

func (s *recruiting) Mine(ctx context.Context, userID, state string) ([]jobs.Posted, error) {
	switch state {
	case "", jobs.StateActive, jobs.StateClosed, jobs.StateExpired, jobs.StatePending:
	default:
		return nil, &jobs.ValidationError{Field: "status", Message: "status must be active, closed or expired"}
	}
	list, err := s.repo.Mine(ctx, userID, state)
	if list == nil {
		list = []jobs.Posted{}
	}
	return list, err
}

func (s *recruiting) Detail(ctx context.Context, id, viewerID string) (*jobs.Detail, error) {
	if uuid.Validate(id) != nil && !legacyID.MatchString(id) {
		return nil, jobs.ErrJobNotFound
	}
	return s.view(ctx, id, false, viewerID)
}

func (s *recruiting) Shared(ctx context.Context, hash, viewerID string) (*jobs.Detail, error) {
	if !shareHash.MatchString(hash) {
		return nil, jobs.ErrJobNotFound
	}
	return s.view(ctx, hash, true, viewerID)
}

func (s *recruiting) view(ctx context.Context, key string, byHash bool, viewerID string) (*jobs.Detail, error) {
	d, err := s.repo.Detail(ctx, key, byHash, viewerID)
	if err != nil {
		return nil, err
	}
	// Views count candidates and visitors, not the job's own team.
	if !d.CanManage {
		if err := s.repo.RecordView(ctx, d.ID); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (s *recruiting) Outbound(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil && !legacyID.MatchString(id) {
		return jobs.ErrJobNotFound
	}
	return s.repo.RecordOutbound(ctx, id)
}

func (s *recruiting) Recruiter(ctx context.Context, id string) (*jobs.Person, int, error) {
	if uuid.Validate(id) != nil {
		return nil, 0, &jobs.NotFoundError{Message: "recruiter not found"}
	}
	return s.repo.Recruiter(ctx, id)
}

func (s *recruiting) Drafts(ctx context.Context, userID string) ([]jobs.Draft, error) {
	list, err := s.repo.Drafts(ctx, userID)
	if list == nil {
		list = []jobs.Draft{}
	}
	return list, err
}

// SaveDraft creates (id "") or updates a draft. Manual: drafts skip validation;
// only the size is capped.
func (s *recruiting) SaveDraft(ctx context.Context, userID, id string, form map[string]any) (*jobs.Draft, error) {
	raw, err := json.Marshal(form)
	if err != nil || len(raw) > maxDraftBytes {
		return nil, &jobs.ValidationError{Field: "form", Message: fmt.Sprintf("drafts can be at most %d KB", maxDraftBytes>>10)}
	}
	if id == "" {
		return s.repo.CreateDraft(ctx, userID, form)
	}
	return s.repo.UpdateDraft(ctx, id, userID, form)
}

func (s *recruiting) DeleteDraft(ctx context.Context, userID, id string) error {
	return s.repo.DeleteDraft(ctx, id, userID)
}

// legacyID matches the sample jobs' slug ids; new jobs use uuids.
var legacyID = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,148}$`)

func randomHash() string {
	b := make([]byte, 16)
	rand.Read(b) // never fails (crypto/rand panics instead)
	return hex.EncodeToString(b)
}

// asJobValidation reports a bad typed term as a job-form validation error.
func asJobValidation(field string, err error) error {
	if reference.IsValidation(err) {
		return &jobs.ValidationError{Field: field, Message: field + ": " + err.Error()}
	}
	return err
}
