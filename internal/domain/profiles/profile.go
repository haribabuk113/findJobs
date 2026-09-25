package profiles

// Profile represents a user profile in the domain
type Profile struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// ProfileDetails represents the full profile details returned to clients
type ProfileDetails struct {
	Profile
	Bio         string   `json:"bio,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Designation string   `json:"designation,omitempty"`
	Location    string   `json:"location,omitempty"`
	AvatarURL   string   `json:"avatar_url,omitempty"`
}

// ProfileRequest represents the request payload for profile operations
type ProfileRequest struct {
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	Bio        string   `json:"bio,omitempty"`
	Skills     []string `json:"skills,omitempty"`
	Designation string  `json:"designation,omitempty"`
	Location   string   `json:"location,omitempty"`
	AvatarURL  string   `json:"avatar_url,omitempty"`
}

// Validate validates the profile request
func (r *ProfileRequest) Validate() error {
	if r.FirstName == "" {
		return ErrFirstNameRequired
	}
	if r.LastName == "" {
		return ErrLastNameRequired
	}
	return nil
}