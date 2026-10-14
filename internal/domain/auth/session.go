package auth

import "time"

// SessionRequest carries the one-time code IAM appended to the app's
// return_to URL (?code=...). The backend redeems it at IAM /token.
type SessionRequest struct {
	Code string `json:"code" binding:"required"`
}

// Validate validates the session request
func (r *SessionRequest) Validate() error {
	if r.Code == "" {
		return ErrCodeRequired
	}
	return nil
}

// User represents a user in the auth response
type User struct {
	ID        string `json:"id"`
	IAMUserID string `json:"iam_user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Status    string `json:"status"`
}

// Authorization represents the authorization context in the auth response
type Authorization struct {
	Roles     []string `json:"roles"`
	Companies []string `json:"companies"`
}

// SessionResponse represents the response for session operations
type SessionResponse struct {
	User          User          `json:"user"`
	Authorization Authorization `json:"authorization"`
	// ExpiresAt is the session's absolute end; the user logs in again after it.
	ExpiresAt time.Time `json:"expires_at"`
}

// AccessTokenResponse hands the current IAM access token to the app's own
// server (e.g. to call IAM admin APIs as this user). IAM enforces admin rights.
type AccessTokenResponse struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// IAMTokens is what IAM returns from /token and /refresh.
type IAMTokens struct {
	AccessToken      string    `json:"access_token"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// IAMClaims is the verified identity inside an IAM access token.
type IAMClaims struct {
	EcosystemID string
	Email       string
	Name        string
	Status      string
}

// Session is a findJobs login session. The IAM refresh token lives here,
// server-side only; the browser holds just the session ID cookie.
type Session struct {
	ID              string
	UserID          string
	RefreshToken    string
	AccessToken     string
	AccessExpiresAt time.Time
	ExpiresAt       time.Time
}
