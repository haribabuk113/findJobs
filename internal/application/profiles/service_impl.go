package profiles

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"slices"
	"time"

	"github.com/google/uuid"

	"findJobs/internal/domain/companies"
	"findJobs/internal/domain/profiles"
	"findJobs/internal/domain/reference"
	"findJobs/internal/pkg/logger"
	"findJobs/internal/ports"
)

// MaxPhotoBytes caps profile photos (the page uploads a 256 px JPEG).
const MaxPhotoBytes = 1 << 20

type service struct {
	repo    ports.ProfileRepository
	terms   TermResolver
	members MembershipRequester
	store   ports.ObjectStore
}

// NewProfileService creates the profile service.
func NewProfileService(repo ports.ProfileRepository, terms TermResolver, members MembershipRequester, store ports.ObjectStore) ProfileService {
	return &service{repo: repo, terms: terms, members: members, store: store}
}

func (s *service) Mine(ctx context.Context, userID string) (*profiles.Profile, error) {
	p, err := s.repo.Get(ctx, userID)
	if !profiles.IsNotFound(err) {
		return p, err
	}
	name, email, err := s.repo.Account(ctx, userID)
	if err != nil {
		return nil, err
	}
	suffix := make([]byte, 2)
	rand.Read(suffix)
	// Not stored until the first save; private-by-default like the page's preview.
	starter := &profiles.Profile{
		Slug: profiles.StarterSlug(name, hex.EncodeToString(suffix)), Name: name, Email: email,
		SalaryCurrency: "INR", Visible: true, Open: true, CurrentSalaryPrivate: true, HourlyPrivate: true,
		ContactPrivate: true, SalaryPrivate: true, ResumePrivate: true,
	}
	starter.EnsureLists()
	return starter, nil
}

func (s *service) Save(ctx context.Context, userID string, in *profiles.Profile) (*profiles.Profile, error) {
	if err := in.Normalize(); err != nil {
		return nil, err
	}
	var names, groups []string
	for _, g := range in.SkillGroups {
		for _, skill := range g.Skills {
			names, groups = append(names, skill), append(groups, g.Name)
		}
	}
	skillTerms, err := s.terms.ResolveTerms(ctx, reference.Skill, names, userID)
	if err != nil {
		return nil, err
	}
	// Two names can resolve to one term (an alias and its master): keep the first.
	skills := make([]ports.ProfileSkill, 0, len(skillTerms))
	seen := map[int64]bool{}
	for i, t := range resolvedInOrder(names, skillTerms) {
		if !seen[t.ID] {
			seen[t.ID] = true
			skills = append(skills, ports.ProfileSkill{SkillID: t.ID, Group: groups[i]})
		}
	}
	roleTerms, err := s.terms.ResolveTerms(ctx, reference.Designation, in.TargetRoles, userID)
	if err != nil {
		return nil, err
	}
	// One term per distinct name, in order; an alias and its master collapse to one id.
	roles := make([]int64, 0, len(roleTerms))
	for _, t := range roleTerms {
		if !slices.Contains(roles, t.ID) {
			roles = append(roles, t.ID)
		}
	}
	if err := s.repo.Save(ctx, userID, in, skills, roles); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, userID)
}

// resolvedInOrder maps each name to its term. ResolveTerms returns one term
// per distinct cleaned name, in first-seen order, so names that clean to the
// same text share a term.
func resolvedInOrder(names []string, terms []reference.Term) []reference.Term {
	out := make([]reference.Term, len(names))
	index := map[string]int{}
	next := 0
	for i, name := range names {
		cleaned, err := reference.CleanName(name)
		if err != nil {
			continue // ResolveTerms already rejected bad names
		}
		j, ok := index[cleaned.Normalized]
		if !ok {
			j, next = next, next+1
			index[cleaned.Normalized] = j
		}
		out[i] = terms[j]
	}
	return out
}

func (s *service) Public(ctx context.Context, slug string) (*profiles.Profile, error) {
	p, err := s.repo.GetPublic(ctx, slug)
	if err != nil {
		return nil, err
	}
	public := p.Public()
	return &public, nil
}

func (s *service) SetPhoto(ctx context.Context, userID, contentType string, data []byte) (*profiles.Profile, error) {
	ext, err := companies.CheckImage(data, contentType, MaxPhotoBytes)
	if err != nil {
		return nil, &profiles.ValidationError{Field: "photo", Message: err.Error()}
	}
	key := "img/" + uuid.NewString() + "." + ext
	if err := s.store.Put(ctx, key, contentType, data); err != nil {
		return nil, err
	}
	old, err := s.repo.SetPhoto(ctx, userID, key)
	if err != nil {
		s.store.Delete(context.WithoutCancel(ctx), key) // not linked: don't leave an orphan
		return nil, err
	}
	if old != "" {
		if err := s.store.Delete(ctx, old); err != nil {
			l := logger.Get()
			l.Warn().Err(err).Str("key", old).Msg("could not delete replaced profile photo")
		}
	}
	return s.repo.Get(ctx, userID)
}

func (s *service) RequestVerification(ctx context.Context, userID string, index int) (*profiles.Profile, error) {
	companyID, title, start, err := s.repo.Experience(ctx, userID, index)
	if err != nil {
		return nil, err
	}
	if companyID == "" {
		return nil, &profiles.ValidationError{Field: "company", Message: "this company isn't on Referise yet, so it can't verify you"}
	}
	started, err := time.Parse("2006-01", start)
	if err != nil {
		return nil, err
	}
	if err := s.members.RequestMembership(ctx, userID, companyID, title, &started); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, userID)
}

func (s *service) Skills(ctx context.Context, userID, signature string) (*profiles.SkillSet, error) {
	skills, err := s.repo.Skills(ctx, userID)
	if err != nil {
		return nil, err
	}
	set := profiles.NewSkillSet(skills)
	if set.Signature == signature {
		return &profiles.SkillSet{Signature: signature}, nil
	}
	return set, nil
}

func (s *service) AddSkill(ctx context.Context, userID string, skillID int64) (*profiles.SkillSet, error) {
	if err := s.repo.AddSkill(ctx, userID, skillID); err != nil {
		return nil, err
	}
	return s.Skills(ctx, userID, "")
}

func (s *service) RemoveSkill(ctx context.Context, userID string, skillID int64) (*profiles.SkillSet, error) {
	if err := s.repo.RemoveSkill(ctx, userID, skillID); err != nil {
		return nil, err
	}
	return s.Skills(ctx, userID, "")
}
