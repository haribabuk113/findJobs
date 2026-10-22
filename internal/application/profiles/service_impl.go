package profiles

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	masterModels "findJobs/internal/domain/master"
	models "findJobs/internal/domain/profiles"
	"findJobs/internal/ports"
	utils "findJobs/internal/utils/common"
	apperror "findJobs/internal/utils/errors"
)

type service struct {
	repo ports.ProfileRepository
	fx   ports.MasterRepository
	opts Options
	// storage ports.CDNProfilePhoto
}

type Options struct {
	BaseURL       string // https://platform.com
	ShortLinkBase string // plat.fm/r/
}

// NewProfileService creates a new profile service using dependency injection
func NewProfileService(repo ports.ProfileRepository) ProfileService {
	return &service{repo: repo}
}

// // GetProfile retrieves a user profile by ID
// func (s *service) GetProfile(ctx context.Context, userID int64) ([]profiles.ProfileDetails, error) {
// 	return s.repo.GetProfile(ctx, userID)
// }

// // UpdateProfile updates a user profile
// func (s *service) UpdateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
// 	if err := request.Validate(); err != nil {
// 		return "", err
// 	}

// 	if err := s.repo.UpdateProfile(ctx, request); err != nil {
// 		return "", err
// 	}

// 	return "Profile updated successfully", nil
// }

// func (s *service) GetCandidateProfile(ctx context.Context, userID int64, userSlug string) (profiles.ProfileDetails, error) {
// 	return s.repo.GetCandidateProfile(ctx, userID, userSlug)
// }

// func (s *service) UpdateCandidateProfile(ctx context.Context, request *profiles.ProfileRequest) (string, error) {
// 	return s.repo.UpdateCandidateProfile(ctx, request)
// }

// // GetProfilePhoto gets a user profile photo url
// func (s *service) GetProfilePhoto(ctx context.Context, userID int64) (profiles.ProfilePhotoResponse, error) {
// 	return s.repo.GetProfilePhoto(ctx, userID)
// }

// // CreateProfilePhoto creates a user profile photo
// func (s *service) CreateProfilePhoto(ctx context.Context, key string, file multipart.File, contentType string, userID int64) (string, error) {
// 	// url, err := s.storage.Upload(ctx, key, file, contentType)
// 	// if err != nil {
// 	// 	err := fmt.Errorf("Upload failed")
// 	// 	return "", err
// 	// }
// 	//
// 	url := key // temporarily
// 	// 5. Save the URL on the candidate record
// 	if err := s.repo.SetProfilePhotoURL(ctx, userID, url); err != nil {
// 		err := fmt.Errorf("Could not save profile photo")
// 		return "", err
// 	}

// 	return url, nil
// }

// func (s *service) GetProfileSlug(ctx context.Context, userID int64) (string, error) {
// 	return s.repo.GetProfileSlug(ctx, userID)
// }

// func (s *service) CreateProfileSlug(ctx context.Context, userID int64) (string, error) {

// 	profile, err := s.repo.GetProfileDetails(ctx, userID)
// 	if err != nil {
// 		err := fmt.Errorf("Could not fetch profile details")
// 		return "", err
// 	}
// 	baseSlug := utilProfiles.BaseSlug(profile.FirstName + " " + profile.LastName)

// 	slug, err := s.repo.UpdateProfileSlug(ctx, baseSlug, userID)
// 	if err != nil {
// 		err := fmt.Errorf("Could not save profile slug")
// 		return "", err
// 	}
// 	return slug, nil
// }

// GetBySlug serves the public profile page.
func (s *service) GetBySlug(ctx context.Context, slug string) (*models.ProfileResponse, error) {
	rec, err := s.repo.GetRecordBySlug(ctx, strings.ToLower(slug))
	if err != nil {
		return nil, err
	}
	return s.toResponse(rec), nil
}

// toResponse maps the persistence aggregate to the public API shape and applies
// the visibility rules (hidden current salary, resume links only when public).
func (s *service) toResponse(rec *models.ProfileRecord) *models.ProfileResponse {
	resp := &models.ProfileResponse{
		CandidateID:     rec.CandidateID,
		FullName:        rec.FullName,
		ProfileSlug:     rec.ProfileSlug,
		ProfilePhotoURL: rec.ProfilePhotoURL,
		Gender:          rec.Gender,
		Pronouns:        append([]string{}, rec.Pronouns...),
		SocialHandles: utils.MapSlice(rec.SocialHandles, func(h models.SocialHandle) models.ProfileSocialHandle {
			return models.ProfileSocialHandle{Platform: h.Platform, URL: h.URL}
		}),
		Preferences: models.PreferencesS{
			OpenToRemote: rec.OpenToRemote,
			WorkTypes:    append([]string{}, rec.WorkTypes...),
			TargetRoles: utils.MapSlice(rec.TargetRoles, func(t models.TargetRole) models.TargetRoleS {
				return models.TargetRoleS{DesignationID: t.DesignationID, Name: t.Name}
			}),
			TargetLocations: utils.MapSlice(rec.TargetLocations, func(t models.TargetLocation) models.TargetLocationS {
				return models.TargetLocationS{LocationID: t.LocationID, Name: t.Name, Level: t.Level}
			}),
		},
		Timeline:    utils.MapSlice(rec.Timeline, TimelineToDTO),
		SkillGroups: utils.MapSlice(rec.SkillGroups, SkillGroupToDTO),
		Contact:     models.Contact{Email: rec.Email, PhoneNumber: rec.Phone},
		// WorkAuthorization: utils.MapSlice(rec.WorkAuthorizations, func(w models.WorkAuthorization) models.WorkAuthorization {
		// 	return models.WorkAuthorization{
		// 		Country: w.Country, AuthorizationType: w.AuthorizationType,
		// 		ValidFrom: w.ValidFrom, ValidUntil: w.ValidUntil, IsIndefinite: w.IsIndefinite,
		// 	}
		// }),
	}

	// headline: null unless at least one part is set
	if rec.HeadlineText != nil || rec.HeadlineDesignationID != nil {
		resp.Headline = &models.Headline{Text: rec.HeadlineText, DesignationID: rec.HeadlineDesignationID}
	}

	if rec.AboutSummary != nil {
		resp.AboutSummary = &models.AboutSummary{Text: *rec.AboutSummary, UpdatedAt: rec.AboutSummaryUpdatedAt}
	}

	// salary: current is privacy-gated, expected is always public
	resp.Salary.Current.IsHidden = rec.Salary.CurrentIsHidden
	if !rec.Salary.CurrentIsHidden && rec.Salary.CurrentAmount != nil {
		resp.Salary.Current.Amount = rec.Salary.CurrentAmount
		resp.Salary.Current.Currency = rec.Salary.CurrentCurrency
	}
	if rec.Salary.ExpectedAmount != nil && rec.Salary.ExpectedCurrency != nil {
		resp.Salary.Expected = &models.SalaryAmountS{
			Amount: *rec.Salary.ExpectedAmount, Currency: *rec.Salary.ExpectedCurrency,
		}
	}

	if rec.PrimaryLocation != nil {
		resp.Contact.PrimaryLocation = &models.LocationRef{
			LocationID: rec.PrimaryLocation.LocationID, Name: rec.PrimaryLocation.Name,
		}
	}

	// resume: links are only exposed when the resume is publicly available
	if rec.ResumeAccessControl != nil {
		res := &models.Resume{AccessControl: *rec.ResumeAccessControl}
		if res.AccessControl == models.ResumePublic {
			view := s.opts.BaseURL + "/r/" + rec.ProfileSlug
			res.ViewURL = &view
			if rec.ResumeShortLink != nil {
				short := s.opts.ShortLinkBase + *rec.ResumeShortLink
				res.PublicShortLink = &short
			}
		}
		resp.Resume = res
	}
	return resp
}

func (s *service) Update(ctx context.Context, candidateID int64, req models.UpdateProfileRequest) error {
	patch := models.CandidatePatch{
		FullName:              trimPtr(req.FullName),
		ProfilePhotoURL:       req.ProfilePhotoURL,
		Gender:                req.Gender,
		HeadlineText:          trimPtr(req.HeadlineText),
		HeadlineDesignationID: req.HeadlineDesignationID,
		AboutSummary:          trimPtr(req.AboutSummary),
	}
	if patch.FullName != nil && len(*patch.FullName) < 2 {
		return fmt.Errorf("Full Name must be at least 2 characters")
	}
	return s.repo.Update(ctx, candidateID, patch)
}

func (s *service) Delete(ctx context.Context, candidateID int64) error {
	return s.repo.SoftDelete(ctx, candidateID)
}

func (s *service) GetPronouns(ctx context.Context, candidateID int64) (models.PronounsResponse, error) {
	p, err := s.repo.ListPronouns(ctx, candidateID)
	return models.PronounsResponse{Pronouns: p}, err
}

func (s *service) ReplacePronouns(ctx context.Context, candidateID int64, req models.PronounsRequest) error {
	seen := map[string]bool{}
	clean := make([]string, 0, len(req.Pronouns))
	for _, p := range req.Pronouns {
		p = strings.ToUpper(strings.TrimSpace(p))
		if p != "" && !seen[p] {
			seen[p] = true
			clean = append(clean, p)
		}
	}
	return s.repo.ReplacePronouns(ctx, candidateID, clean)
}

func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}

func (s *service) ListSocial(ctx context.Context, candidateID int64) ([]models.SocialHandleResponse, error) {
	hs, err := s.repo.ListSocial(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(hs, func(h models.SocialHandle) models.SocialHandleResponse {
		return models.SocialHandleResponse{ID: h.ID, Platform: h.Platform, URL: h.URL, ClickCount: h.ClickCount}
	}), nil
}

func (s *service) CreateSocial(ctx context.Context, candidateID int64, req models.SocialHandleRequest) (int64, error) {
	return s.repo.CreateSocial(ctx, models.SocialHandle{
		CandidateID: candidateID, Platform: normalizePlatform(req.Platform), URL: req.URL,
	})
}

func (s *service) UpdateSocial(ctx context.Context, candidateID, id int64, req models.SocialHandleRequest) error {
	return s.repo.UpdateSocial(ctx, models.SocialHandle{
		ID: id, CandidateID: candidateID, Platform: normalizePlatform(req.Platform), URL: req.URL,
	})
}

func (s *service) DeleteSocial(ctx context.Context, candidateID, id int64) error {
	return s.repo.DeleteSocial(ctx, id, candidateID)
}

// TrackClick is public: records an outgoing click on someone's profile link.
func (s *service) TrackClickSocial(ctx context.Context, slug, platform string) error {
	return s.repo.IncrementClickSocial(ctx, strings.ToLower(slug), normalizePlatform(platform))
}

func normalizePlatform(p string) string { return strings.ToUpper(strings.TrimSpace(p)) }

func (s *service) GetSalary(ctx context.Context, candidateID int64) (*models.SalaryResponse, error) {
	m, err := s.repo.GetSalary(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return &models.SalaryResponse{
		CurrentAmount: m.CurrentAmount, CurrentCurrency: m.CurrentCurrency, CurrentIsHidden: m.CurrentIsHidden,
		ExpectedAmount: m.ExpectedAmount, ExpectedCurrency: m.ExpectedCurrency,
	}, nil
}

func (s *service) PutSalary(ctx context.Context, candidateID int64, req models.SalaryRequest) error {
	if (req.CurrentAmount == nil) != (req.CurrentCurrency == nil) ||
		(req.ExpectedAmount == nil) != (req.ExpectedCurrency == nil) {
		return fmt.Errorf("Amount and Currency must be provided together")
	}
	curCur, expCur := upperPtr(req.CurrentCurrency), upperPtr(req.ExpectedCurrency)

	// INR is stored even when the current salary is hidden, so recruiters can still filter by range.
	curINR, err := s.toINR(ctx, req.CurrentAmount, curCur)
	if err != nil {
		return err
	}
	expINR, err := s.toINR(ctx, req.ExpectedAmount, expCur)
	if err != nil {
		return err
	}
	return s.repo.UpsertSalary(ctx, models.Salary{
		CandidateID:   candidateID,
		CurrentAmount: req.CurrentAmount, CurrentCurrency: curCur, CurrentAmountINR: curINR,
		CurrentIsHidden: req.CurrentIsHidden,
		ExpectedAmount:  req.ExpectedAmount, ExpectedCurrency: expCur, ExpectedAmountINR: expINR,
	})
}

func (s *service) DeleteSalary(ctx context.Context, candidateID int64) error {
	return s.repo.DeleteSalary(ctx, candidateID)
}

func (s *service) toINR(ctx context.Context, amount *float64, currency *string) (*float64, error) {
	if amount == nil || currency == nil {
		return nil, nil
	}
	if *currency == "INR" {
		v := *amount
		return &v, nil
	}
	rate, err := s.fx.LatestRate(ctx, *currency)
	if err != nil {
		if apperror.IsNotFound(err) {
			return nil, &apperror.Error{Status: http.StatusUnprocessableEntity, Message: "no FX rate available for " + *currency}

		}
		return nil, err
	}
	v := *amount * rate
	return &v, nil
}

func upperPtr(s *string) *string {
	if s == nil {
		return nil
	}
	u := strings.ToUpper(*s)
	return &u
}

func (s *service) GetPreference(ctx context.Context, candidateID int64) (*models.PreferencesS, error) {
	p, err := s.repo.GetPreference(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return &models.PreferencesS{
		OpenToRemote: p.OpenToRemote,
		WorkTypes:    p.WorkTypes,
		TargetRoles: utils.MapSlice(p.TargetRoles, func(t models.TargetRole) models.TargetRoleS {
			return models.TargetRoleS{DesignationID: t.DesignationID, Name: t.Name}
		}),
		TargetLocations: utils.MapSlice(p.TargetLocations, func(t models.TargetLocation) models.TargetLocationS {
			return models.TargetLocationS{LocationID: t.LocationID, Name: t.Name, Level: t.Level}
		}),
	}, nil
}

func (s *service) PutPreference(ctx context.Context, candidateID int64, req models.PreferencesRequest) error {
	return s.repo.UpsertPreference(ctx, candidateID, req.OpenToRemote, req.WorkTypes)
}

func (s *service) PutTargetRoles(ctx context.Context, candidateID int64, req models.TargetRolesRequest) error {
	return s.repo.ReplaceTargetRoles(ctx, candidateID, req.DesignationIDs)
}

func (s *service) PutTargetLocations(ctx context.Context, candidateID int64, req models.TargetLocationsRequest) error {
	return s.repo.ReplaceTargetLocations(ctx, candidateID, req.LocationIDs)
}

func (s *service) ListTimeline(ctx context.Context, candidateID int64) ([]models.TimelineItem, error) {
	es, err := s.repo.ListTimeline(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(es, TimelineToDTO), nil
}

func (s *service) CreateTimeline(ctx context.Context, candidateID int64, req models.TimelineRequest) (int64, error) {
	if err := validateTimeline(req); err != nil {
		return 0, err
	}
	t := toTimelineModel(req)
	t.CandidateID = candidateID
	return s.repo.CreateTimeline(ctx, t)
}

func (s *service) UpdateTimeline(ctx context.Context, candidateID, id int64, req models.TimelineRequest) error {
	if err := validateTimeline(req); err != nil {
		return err
	}
	t := toTimelineModel(req)
	t.TimelineID, t.CandidateID = id, candidateID
	return s.repo.UpdateTimeline(ctx, t)
}

func (s *service) DeleteTimeline(ctx context.Context, candidateID, id int64) error {
	return s.repo.DeleteTimeline(ctx, id, candidateID)
}

func (s *service) RequestTimelineVerification(ctx context.Context, candidateID, id int64) error {
	return s.repo.RequestTimelineVerification(ctx, id, candidateID)
}

// Review is called by a company admin. TODO: also check adminUserID belongs to
// the role's company once the company-admin table exists.
func (s *service) ReviewTimeline(ctx context.Context, id int64, approve bool, adminUserID int64) error {
	return s.repo.ReviewTimeline(ctx, id, approve, adminUserID)
}

func toTimelineModel(r models.TimelineRequest) models.Timeline {
	t := models.Timeline{
		CompanyID:        r.CompanyID,
		DesignationTitle: strings.TrimSpace(r.DesignationTitle),
		StartDate:        r.StartDate,
		EndDate:          r.EndDate,
		IsCurrent:        r.IsCurrent,
		Description:      r.Description,
	}
	// A registered company wins; the free-text name is only kept when there is no company_id.
	if r.CompanyID == nil {
		t.CompanyNameRaw = trimPtr(r.CompanyNameRaw)
	}
	return t
}

// validateTimeline enforces the cross-field rules binding tags cannot express.
func validateTimeline(r models.TimelineRequest) error {
	if r.CompanyID == nil && (r.CompanyNameRaw == nil || strings.TrimSpace(*r.CompanyNameRaw) == "") {
		return fmt.Errorf("company_id or company_name_raw is required")
	}
	if strings.TrimSpace(r.DesignationTitle) == "" {
		return fmt.Errorf("designation_title is required")
	}
	if r.IsCurrent && r.EndDate != nil {
		return fmt.Errorf("a current role cannot have an end_date")
	}
	if !r.IsCurrent && r.EndDate == nil {
		return fmt.Errorf("end_date is required unless is_current is true")
	}
	start, err := time.Parse(models.DateLayout, r.StartDate)
	if err != nil {
		return fmt.Errorf("start_date must be YYYY-MM-DD")
	}
	if r.EndDate != nil {
		end, err := time.Parse(models.DateLayout, *r.EndDate)
		if err != nil {
			return fmt.Errorf("end_date must be YYYY-MM-DD")
		}
		if end.Before(start) {
			return fmt.Errorf("end_date cannot be before start_date")
		}
	}
	return nil
}

func TimelineToDTO(e models.TimelineEntry) models.TimelineItem {
	item := models.TimelineItem{
		TimelineID:         e.TimelineID,
		DesignationTitle:   e.DesignationTitle,
		StartDate:          e.StartDate,
		EndDate:            e.EndDate,
		IsCurrent:          e.IsCurrent,
		Description:        e.Description,
		VerificationStatus: e.VerificationStatus,
	}
	if e.Company != nil {
		item.Company = &models.TimelineCompany{CompanyID: e.Company.CompanyID, Name: e.Company.Name}
	}
	return item
}

func SkillGroupToDTO(g models.SkillGroup) models.SkillGroup {
	return models.SkillGroup{
		GroupID:      g.GroupID,
		GroupName:    g.GroupName,
		DisplayOrder: g.DisplayOrder,
		Skills: utils.MapSlice(g.Skills, func(k models.SkillChip) models.SkillChip {
			return models.SkillChip{SkillID: k.SkillID, Name: k.Name, DisplayOrder: k.DisplayOrder}
		}),
	}
}

func (s *service) ListSkillGroup(ctx context.Context, candidateID int64) ([]models.SkillGroup, error) {
	gs, err := s.repo.ListSkillGroups(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(gs, SkillGroupToDTO), nil
}

func (s *service) CreateSkillGroup(ctx context.Context, candidateID int64, req models.SkillGroupRequest) (int64, error) {
	return s.repo.CreateSkillGroup(ctx, candidateID, strings.TrimSpace(req.GroupName), req.DisplayOrder)
}

func (s *service) UpdateSkillGroup(ctx context.Context, candidateID, groupID int64, req models.SkillGroupRequest) error {
	return s.repo.UpdateSkillGroup(ctx, groupID, candidateID, strings.TrimSpace(req.GroupName), req.DisplayOrder)
}

func (s *service) DeleteSkillGroup(ctx context.Context, candidateID, groupID int64) error {
	return s.repo.DeleteSkillGroup(ctx, groupID, candidateID)
}

// AddSkill attaches a master skill, or creates a custom one (pending admin review) from a name.
func (s *service) AddSkill(ctx context.Context, candidateID, groupID int64, req models.AddSkillRequest) (*models.AddSkillResponse, error) {
	owns, err := s.repo.OwnsGroup(ctx, groupID, candidateID)
	if err != nil {
		return nil, err
	}
	if !owns {
		return nil, &apperror.Error{Status: http.StatusNotFound, Message: "skill group not found"}
	}

	var skillID int64
	switch {
	case req.SkillID != nil:
		skillID = *req.SkillID
	case req.Name != nil && strings.TrimSpace(*req.Name) != "":
		entry, err := s.fx.FindOrCreatePending(ctx, masterModels.MasterSkills, strings.TrimSpace(*req.Name), candidateID)
		if err != nil {
			return nil, err
		}
		skillID = entry.ID
	default:
		return nil, fmt.Errorf("Skill id or name is required")
	}

	itemID, err := s.repo.AddSkill(ctx, groupID, skillID, req.DisplayOrder)
	if err != nil {
		return nil, err
	}
	return &models.AddSkillResponse{ID: itemID, SkillID: skillID}, nil
}

func (s *service) UpdateSkill(ctx context.Context, candidateID, groupID, skillID int64, req models.UpdateSkillItemRequest) error {
	return s.repo.UpdateSkill(ctx, groupID, skillID, candidateID, req.DisplayOrder)
}

func (s *service) RemoveSkill(ctx context.Context, candidateID, groupID, skillID int64) error {
	return s.repo.RemoveSkill(ctx, groupID, skillID, candidateID)
}

func (s *service) GetContact(ctx context.Context, candidateID int64) (*models.ContactResponse, error) {
	c, err := s.repo.GetContact(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return &models.ContactResponse{Email: c.Email, PhoneNumber: c.PhoneNumber, PrimaryLocationID: c.PrimaryLocationID}, nil
}

func (s *service) PutContact(ctx context.Context, candidateID int64, req models.ContactRequest) error {
	return s.repo.UpsertContact(ctx, models.ContactS{
		CandidateID:       candidateID,
		Email:             strings.TrimSpace(req.Email),
		PhoneNumber:       trimPtr(req.PhoneNumber),
		PrimaryLocationID: req.PrimaryLocationID,
	})
}

func (s *service) GetResume(ctx context.Context, candidateID int64) (*models.ResumeResponse, error) {
	r, err := s.repo.GetResume(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return &models.ResumeResponse{
		FileURL: r.FileURL, FileSizeBytes: r.FileSizeBytes, AccessControl: r.AccessControl,
		PublicShortLink: r.PublicShortLink, UploadedAt: r.UploadedAt,
	}, nil
}

func (s *service) PutResume(ctx context.Context, candidateID int64, req models.ResumeRequest) error {
	access := req.AccessControl
	if access == "" {
		access = models.ResumePublic
	}
	short := req.PublicShortLink
	if access != models.ResumePublic { // short links only exist for public resumes
		short = nil
	}
	return s.repo.UpsertResume(ctx, models.ResumeS{
		CandidateID: candidateID, FileURL: req.FileURL, FileSizeBytes: req.FileSizeBytes,
		AccessControl: access, PublicShortLink: short,
	})
}

func (s *service) DeleteResume(ctx context.Context, candidateID int64) error {
	return s.repo.DeleteResume(ctx, candidateID)
}

func (s *service) ListResumeGrants(ctx context.Context, candidateID int64) ([]models.GrantResponse, error) {
	gs, err := s.repo.ListResumeGrants(ctx, candidateID)
	if err != nil {
		return nil, err
	}
	return utils.MapSlice(gs, func(g models.ResumeGrant) models.GrantResponse {
		return models.GrantResponse{ID: g.ID, RecruiterID: g.RecruiterID, JobPostingID: g.JobPostingID, GrantedAt: g.GrantedAt}
	}), nil
}

func (s *service) RevokeResumeGrant(ctx context.Context, candidateID, id int64) error {
	return s.repo.RevokeResumeGrant(ctx, id, candidateID)
}

func (s *service) CreateResumeGrant(ctx context.Context, req models.CreateGrantRequest) error {
	return s.repo.CreateResumeGrant(ctx, models.ResumeGrant{
		CandidateID: req.CandidateID, RecruiterID: req.RecruiterID, JobPostingID: req.JobPostingID,
	})
}
