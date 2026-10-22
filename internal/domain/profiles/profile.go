package profiles

import "time"

// // Profile represents a user profile in the domain
// type Profile struct {
// 	ID        int64  `json:"id"`
// 	Email     string `json:"email"`
// 	FirstName string `json:"first_name"`
// 	LastName  string `json:"last_name"`
// 	CreatedAt int64  `json:"created_at"`
// 	UpdatedAt int64  `json:"updated_at"`
// }

// // ProfileDetails represents the full profile details returned to clients
// type ProfileDetails struct {
// 	Profile
// 	Bio         string   `json:"bio,omitempty"`
// 	Skills      []string `json:"skills,omitempty"`
// 	Designation string   `json:"designation,omitempty"`
// 	Location    string   `json:"location,omitempty"`
// 	AvatarURL   string   `json:"avatar_url,omitempty"`
// }

// // ProfileRequest represents the request payload for profile operations
// type ProfileRequest struct {
// 	FirstName   string   `json:"first_name"`
// 	LastName    string   `json:"last_name"`
// 	Bio         string   `json:"bio,omitempty"`
// 	Skills      []string `json:"skills,omitempty"`
// 	Designation string   `json:"designation,omitempty"`
// 	Location    string   `json:"location,omitempty"`
// 	AvatarURL   string   `json:"avatar_url,omitempty"`
// }

// // Validate validates the profile request
// func (r *ProfileRequest) Validate() error {
// 	if r.FirstName == "" {
// 		return ErrFirstNameRequired
// 	}
// 	if r.LastName == "" {
// 		return ErrLastNameRequired
// 	}
// 	return nil
// }

// // ProfileRequest represents the request payload for profile operations
// type ProfilePhotoResponse struct {
// 	AvatarURL string `json:"avatar_url,omitempty"`
// }

// // ---------- GET /profile/{slug} response ----------

// type ProfileResponse struct {
// 	CandidateID       int64               `json:"candidate_id"`
// 	FullName          string              `json:"full_name"`
// 	ProfileSlug       string              `json:"profile_slug"`
// 	ProfilePhotoURL   *string             `json:"profile_photo_url"`
// 	Gender            *string             `json:"gender"`
// 	Pronouns          []string            `json:"pronouns"`
// 	Headline          *Headline           `json:"headline"`
// 	AboutSummary      *AboutSummary       `json:"about_summary"`
// 	SocialHandles     []SocialHandle      `json:"social_handles"`
// 	Salary            Salary              `json:"salary"`
// 	Preferences       Preferences         `json:"preferences"`
// 	Timeline          []TimelineEntry     `json:"timeline"`
// 	SkillGroups       []SkillGroup        `json:"skill_groups"`
// 	Contact           Contact             `json:"contact"`
// 	Resume            *Resume             `json:"resume"`
// 	WorkAuthorization []WorkAuthorization `json:"work_authorization"`
// }

// type Headline struct {
// 	Text          *string `json:"text"`
// 	DesignationID *int64  `json:"designation_id"`
// }

// type AboutSummary struct {
// 	Text      string     `json:"text"`
// 	UpdatedAt *time.Time `json:"updated_at"`
// }

// type SocialHandle struct {
// 	Platform string `json:"platform"`
// 	URL      string `json:"url"`
// }

// type Salary struct {
// 	Current  SalaryCurrent `json:"current"`
// 	Expected *SalaryAmount `json:"expected"` // null when not set
// }

// // When is_hidden=true, amount/currency are omitted entirely.
// type SalaryCurrent struct {
// 	IsHidden bool     `json:"is_hidden"`
// 	Amount   *float64 `json:"amount,omitempty"`
// 	Currency *string  `json:"currency,omitempty"`
// }

// type SalaryAmount struct {
// 	Amount   float64 `json:"amount"`
// 	Currency string  `json:"currency"`
// }

// type Preferences struct {
// 	TargetRoles     []TargetRole     `json:"target_roles"`
// 	TargetLocations []TargetLocation `json:"target_locations"`
// 	OpenToRemote    bool             `json:"open_to_remote"`
// 	WorkTypes       []string         `json:"work_types"`
// }

// type TargetRole struct {
// 	DesignationID int64  `json:"designation_id"`
// 	Name          string `json:"name"`
// }

// type TargetLocation struct {
// 	LocationID int64  `json:"location_id"`
// 	Name       string `json:"name"`
// 	Level      string `json:"level"`
// }

// type TimelineEntry struct {
// 	TimelineID         int64            `json:"timeline_id"`
// 	Company            *TimelineCompany `json:"company"`
// 	DesignationTitle   string           `json:"designation_title"`
// 	StartDate          string           `json:"start_date"`
// 	EndDate            *string          `json:"end_date"`
// 	IsCurrent          bool             `json:"is_current"`
// 	Description        *string          `json:"description"`
// 	VerificationStatus string           `json:"verification_status"`
// }

// // CompanyID is null when the company is not registered (free-text name only).
// type TimelineCompany struct {
// 	CompanyID *int64 `json:"company_id"`
// 	Name      string `json:"name"`
// }

// type SkillGroup struct {
// 	GroupID      int64       `json:"group_id"`
// 	GroupName    string      `json:"group_name"`
// 	DisplayOrder int         `json:"display_order"`
// 	Skills       []SkillChip `json:"skills"`
// }

// type SkillChip struct {
// 	SkillID      int64  `json:"skill_id"`
// 	Name         string `json:"name"`
// 	DisplayOrder int    `json:"display_order"`
// }

// type Contact struct {
// 	Email           string       `json:"email"`
// 	PhoneNumber     *string      `json:"phone_number"`
// 	PrimaryLocation *LocationRef `json:"primary_location"`
// }

// type LocationRef struct {
// 	LocationID int64  `json:"location_id"`
// 	Name       string `json:"name"`
// }

// type Resume struct {
// 	AccessControl   string  `json:"access_control"`
// 	ViewURL         *string `json:"view_url,omitempty"`
// 	PublicShortLink *string `json:"public_short_link,omitempty"`
// }

// type WorkAuthorization struct {
// 	Country           string  `json:"country"`
// 	AuthorizationType string  `json:"authorization_type"`
// 	ValidFrom         *string `json:"valid_from"`
// 	ValidUntil        *string `json:"valid_until"`
// 	IsIndefinite      bool    `json:"is_indefinite"`
// }

type ProfileResponse struct {
	CandidateID     int64                 `json:"candidate_id"`
	FullName        string                `json:"full_name"`
	ProfileSlug     string                `json:"profile_slug"`
	ProfilePhotoURL *string               `json:"profile_photo_url"`
	Gender          *string               `json:"gender"`
	Pronouns        []string              `json:"pronouns"`
	Headline        *Headline             `json:"headline"`
	AboutSummary    *AboutSummary         `json:"about_summary"`
	SocialHandles   []ProfileSocialHandle `json:"social_handles"`
	Salary          SalaryS               `json:"salary"`
	Preferences     PreferencesS          `json:"preferences"`
	Timeline        []TimelineItem        `json:"timeline"`
	SkillGroups     []SkillGroup          `json:"skill_groups"`
	Contact         Contact               `json:"contact"`
	Resume          *Resume               `json:"resume"`
	// WorkAuthorization []WorkAuthorization   `json:"work_authorization"`
}

type Headline struct {
	Text          *string `json:"text"`
	DesignationID *int64  `json:"designation_id"`
}

type AboutSummary struct {
	Text      string     `json:"text"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type ProfileSocialHandle struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type SalaryS struct {
	Current  SalaryCurrentS `json:"current"`
	Expected *SalaryAmountS `json:"expected"`
}

// When is_hidden is true, amount and currency are omitted entirely.
type SalaryCurrentS struct {
	IsHidden bool     `json:"is_hidden"`
	Amount   *float64 `json:"amount,omitempty"`
	Currency *string  `json:"currency,omitempty"`
}

type SalaryAmountS struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type PreferencesS struct {
	TargetRoles     []TargetRoleS     `json:"target_roles"`
	TargetLocations []TargetLocationS `json:"target_locations"`
	OpenToRemote    bool              `json:"open_to_remote"`
	WorkTypes       []string          `json:"work_types"`
}

type TargetRoleS struct {
	DesignationID int64  `json:"designation_id"`
	Name          string `json:"name"`
}

type TargetLocationS struct {
	LocationID int64  `json:"location_id"`
	Name       string `json:"name"`
	Level      string `json:"level"`
}

type TimelineItem struct {
	TimelineID         int64            `json:"timeline_id"`
	Company            *TimelineCompany `json:"company"`
	DesignationTitle   string           `json:"designation_title"`
	StartDate          string           `json:"start_date"`
	EndDate            *string          `json:"end_date"`
	IsCurrent          bool             `json:"is_current"`
	Description        *string          `json:"description"`
	VerificationStatus string           `json:"verification_status"`
}

// CompanyID is null when the company is not registered (free-text name only).
type TimelineCompany struct {
	CompanyID *int64 `json:"company_id"`
	Name      string `json:"name"`
}

type SkillGroup struct {
	GroupID      int64       `json:"group_id"`
	GroupName    string      `json:"group_name"`
	DisplayOrder int         `json:"display_order"`
	Skills       []SkillChip `json:"skills"`
}

type SkillChip struct {
	SkillID      int64  `json:"skill_id"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"display_order"`
}

type Contact struct {
	Email           string       `json:"email"`
	PhoneNumber     *string      `json:"phone_number"`
	PrimaryLocation *LocationRef `json:"primary_location"`
}

type ContactS struct {
	CandidateID       int64
	Email             string
	PhoneNumber       *string
	PrimaryLocationID *int64
}

type LocationRef struct {
	LocationID int64  `json:"location_id"`
	Name       string `json:"name"`
}

type Resume struct {
	AccessControl   string  `json:"access_control"`
	ViewURL         *string `json:"view_url,omitempty"`
	PublicShortLink *string `json:"public_short_link,omitempty"`
}

// ---- resume

const (
	ResumePublic              = "PUBLICLY_AVAILABLE"
	ResumeApplicationGridOnly = "APPLICATION_GRID_ONLY"
)

// type WorkAuthorization struct {
// 	Country           string  `json:"country"`
// 	AuthorizationType string  `json:"authorization_type"`
// 	ValidFrom         *string `json:"valid_from"`
// 	ValidUntil        *string `json:"valid_until"`
// 	IsIndefinite      bool    `json:"is_indefinite"`
// }

// ---------- PATCH /me/profile ----------

// PATCH semantics: omitted / null fields are left unchanged.
type UpdateProfileRequest struct {
	FullName              *string `json:"full_name" binding:"omitempty,min=2,max=100"`
	ProfilePhotoURL       *string `json:"profile_photo_url" binding:"omitempty,url"`
	Gender                *string `json:"gender" binding:"omitempty,oneof=MALE FEMALE NON_BINARY PREFER_NOT_TO_SAY"`
	HeadlineText          *string `json:"headline_text" binding:"omitempty,max=200"`
	HeadlineDesignationID *int64  `json:"headline_designation_id" binding:"omitempty,gt=0"`
	AboutSummary          *string `json:"about_summary" binding:"omitempty,max=2000"`
}

// ---------- pronouns ----------

type PronounsRequest struct {
	Pronouns []string `json:"pronouns" binding:"required,max=10,dive,min=1,max=30"`
}

type PronounsResponse struct {
	Pronouns []string `json:"pronouns"`
}

// ProfileRecord is the read-side aggregate behind GET /profile/{slug}.
type ProfileRecord struct {
	CandidateID           int64
	FullName              string
	ProfileSlug           string
	ProfilePhotoURL       *string
	Gender                *string
	HeadlineText          *string
	HeadlineDesignationID *int64
	AboutSummary          *string
	AboutSummaryUpdatedAt *time.Time

	Pronouns        []string
	SocialHandles   []SocialHandle
	Salary          Salary
	TargetRoles     []TargetRole
	TargetLocations []TargetLocation
	OpenToRemote    bool
	WorkTypes       []string
	Timeline        []TimelineEntry
	SkillGroups     []SkillGroup

	Email           string
	Phone           *string
	PrimaryLocation *LocationRef

	ResumeAccessControl *string
	ResumeShortLink     *string

	// WorkAuthorizations []WorkAuthorization
}

// CandidatePatch: nil fields are left unchanged.
type CandidatePatch struct {
	FullName              *string
	ProfilePhotoURL       *string
	Gender                *string
	HeadlineText          *string
	HeadlineDesignationID *int64
	AboutSummary          *string
}

// ---- social

type SocialHandle struct {
	ID          int64  `json:"id"`
	CandidateID int64  `json:"-"`
	Platform    string `json:"platform"`
	URL         string `json:"url"`
	ClickCount  int64  `json:"click_count"`
}

// ---------- social handles ----------

type SocialHandleRequest struct {
	Platform string `json:"platform" binding:"required,max=50"`
	URL      string `json:"url" binding:"required,url,max=2048"`
}

type SocialHandleResponse struct {
	ID         int64  `json:"id"`
	Platform   string `json:"platform"`
	URL        string `json:"url"`
	ClickCount int64  `json:"click_count"`
}

// ---------- salary ----------

type Salary struct {
	CandidateID       int64
	CurrentAmount     *float64
	CurrentCurrency   *string
	CurrentAmountINR  *float64
	CurrentIsHidden   bool
	ExpectedAmount    *float64
	ExpectedCurrency  *string
	ExpectedAmountINR *float64
}

type SalaryRequest struct {
	CurrentAmount    *float64 `json:"current_amount" binding:"omitempty,gte=0"`
	CurrentCurrency  *string  `json:"current_currency" binding:"omitempty,len=3,alpha"`
	CurrentIsHidden  bool     `json:"current_is_hidden"`
	ExpectedAmount   *float64 `json:"expected_amount" binding:"omitempty,gte=0"`
	ExpectedCurrency *string  `json:"expected_currency" binding:"omitempty,len=3,alpha"`
}

// Owner view: amounts are returned even when current_is_hidden is true.
type SalaryResponse struct {
	CurrentAmount    *float64 `json:"current_amount"`
	CurrentCurrency  *string  `json:"current_currency"`
	CurrentIsHidden  bool     `json:"current_is_hidden"`
	ExpectedAmount   *float64 `json:"expected_amount"`
	ExpectedCurrency *string  `json:"expected_currency"`
}

// ---------- preferences ----------

type Preferences struct {
	OpenToRemote    bool
	WorkTypes       []string
	TargetRoles     []TargetRole
	TargetLocations []TargetLocation
}

type TargetRole struct {
	DesignationID int64  `json:"designation_id"`
	Name          string `json:"name"`
}

type TargetLocation struct {
	LocationID int64  `json:"location_id"`
	Name       string `json:"name"`
	Level      string `json:"level"`
}

type PreferencesRequest struct {
	OpenToRemote bool     `json:"open_to_remote"`
	WorkTypes    []string `json:"work_types" binding:"required,dive,oneof=FULL_TIME CONTRACT PART_TIME GIG ADVISORY VOLUNTEERING"`
}

type TargetRolesRequest struct {
	DesignationIDs []int64 `json:"designation_ids" binding:"required,max=50,dive,gt=0"`
}

type TargetLocationsRequest struct {
	LocationIDs []int64 `json:"location_ids" binding:"required,max=50,dive,gt=0"`
}

// ---------- timeline ----------

type TimelineRequest struct {
	CompanyID        *int64  `json:"company_id" binding:"omitempty,gt=0"`
	CompanyNameRaw   *string `json:"company_name_raw" binding:"omitempty,max=200"`
	DesignationTitle string  `json:"designation_title" binding:"required,max=150"`
	StartDate        string  `json:"start_date" binding:"required,datetime=2006-01-02"`
	EndDate          *string `json:"end_date" binding:"omitempty,datetime=2006-01-02"`
	IsCurrent        bool    `json:"is_current"`
	Description      *string `json:"description" binding:"omitempty,max=5000"`
}

type IDResponse struct {
	ID int64 `json:"id"`
}

type TimelineIDResponse struct {
	TimelineID int64 `json:"timeline_id"`
}

type VerificationReviewRequest struct {
	Approve bool `json:"approve"`
}

const (
	VerificationUnverified = "UNVERIFIED"
	VerificationPending    = "PENDING_VERIFICATION"
	VerificationVerified   = "VERIFIED"
	VerificationRejected   = "REJECTED"
)

const DateLayout = "2006-01-02"

// Timeline is the write model. Dates are "YYYY-MM-DD" strings.
type Timeline struct {
	TimelineID       int64
	CandidateID      int64
	CompanyID        *int64
	CompanyNameRaw   *string
	DesignationTitle string
	StartDate        string
	EndDate          *string
	IsCurrent        bool
	Description      *string
}

// TimelineEntry is the read model (company already resolved).
type TimelineEntry struct {
	TimelineID         int64       `json:"timeline_id"`
	Company            *CompanyRef `json:"company"`
	DesignationTitle   string      `json:"designation_title"`
	StartDate          string      `json:"start_date"`
	EndDate            *string     `json:"end_date"`
	IsCurrent          bool        `json:"is_current"`
	Description        *string     `json:"description"`
	VerificationStatus string      `json:"verification_status"`
}

// CompanyRef.CompanyID is nil when the company is free text only.
type CompanyRef struct {
	CompanyID *int64 `json:"company_id"`
	Name      string `json:"name"`
}

// ---------- skill groups ----------

type SkillGroupRequest struct {
	GroupName    string `json:"group_name" binding:"required,max=100"`
	DisplayOrder int    `json:"display_order" binding:"gte=0"`
}

type SkillGroupIDResponse struct {
	GroupID int64 `json:"group_id"`
}

// Pass skill_id for a master skill, or name to create a custom (pending review) one.
type AddSkillRequest struct {
	SkillID      *int64  `json:"skill_id" binding:"omitempty,gt=0"`
	Name         *string `json:"name" binding:"omitempty,max=150"`
	DisplayOrder int     `json:"display_order" binding:"gte=0"`
}

type AddSkillResponse struct {
	ID      int64 `json:"id"`
	SkillID int64 `json:"skill_id"`
}

type UpdateSkillItemRequest struct {
	DisplayOrder int `json:"display_order" binding:"gte=0"`
}

// ---------- contact ----------

type ContactRequest struct {
	Email             string  `json:"email" binding:"required,email,max=255"`
	PhoneNumber       *string `json:"phone_number" binding:"omitempty,max=30"`
	PrimaryLocationID *int64  `json:"primary_location_id" binding:"omitempty,gt=0"`
}

type ContactResponse struct {
	Email             string  `json:"email"`
	PhoneNumber       *string `json:"phone_number"`
	PrimaryLocationID *int64  `json:"primary_location_id"`
}

// ---------- resume ----------

type ResumeS struct {
	CandidateID     int64
	FileURL         string
	FileSizeBytes   int64
	AccessControl   string
	PublicShortLink *string
	UploadedAt      time.Time
}

type ResumeRequest struct {
	FileURL         string  `json:"file_url" binding:"required,url"`
	FileSizeBytes   int64   `json:"file_size_bytes" binding:"required,gt=0,lte=5242880"`
	AccessControl   string  `json:"access_control" binding:"omitempty,oneof=PUBLICLY_AVAILABLE APPLICATION_GRID_ONLY"`
	PublicShortLink *string `json:"public_short_link" binding:"omitempty,alphanum,max=50"` // code only
}

type ResumeResponse struct {
	FileURL         string    `json:"file_url"`
	FileSizeBytes   int64     `json:"file_size_bytes"`
	AccessControl   string    `json:"access_control"`
	PublicShortLink *string   `json:"public_short_link"`
	UploadedAt      time.Time `json:"uploaded_at"`
}

type ResumeGrant struct {
	ID           int64
	CandidateID  int64
	RecruiterID  int64
	JobPostingID int64
	GrantedAt    time.Time
}

type GrantResponse struct {
	ID           int64     `json:"id"`
	RecruiterID  int64     `json:"recruiter_id"`
	JobPostingID int64     `json:"job_posting_id"`
	GrantedAt    time.Time `json:"granted_at"`
}

// Called service-to-service when a candidate applies to a job.
type CreateGrantRequest struct {
	CandidateID  int64 `json:"candidate_id" binding:"required,gt=0"`
	RecruiterID  int64 `json:"recruiter_id" binding:"required,gt=0"`
	JobPostingID int64 `json:"job_posting_id" binding:"required,gt=0"`
}

// ---------- work authorization ----------

type WorkAuthorizationRequest struct {
	Country           string  `json:"country" binding:"required,max=100"`
	AuthorizationType string  `json:"authorization_type" binding:"required,max=100"`
	ValidFrom         *string `json:"valid_from" binding:"omitempty,datetime=2006-01-02"`
	ValidUntil        *string `json:"valid_until" binding:"omitempty,datetime=2006-01-02"`
	IsIndefinite      bool    `json:"is_indefinite"`
}

type WorkAuthorizationResponse struct {
	ID                int64   `json:"id"`
	Country           string  `json:"country"`
	AuthorizationType string  `json:"authorization_type"`
	ValidFrom         *string `json:"valid_from"`
	ValidUntil        *string `json:"valid_until"`
	IsIndefinite      bool    `json:"is_indefinite"`
}
