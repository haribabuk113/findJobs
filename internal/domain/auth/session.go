package auth

// SessionRequest represents the request payload for creating/resolving a session
type SessionRequest struct {
	AppCode string `json:"appcode" binding:"required"`
}

// Validate validates the session request
func (r *SessionRequest) Validate() error {
	if r.AppCode == "" {
		return ErrAppCodeRequired
	}
	return nil
}

// User represents a user in the auth response
type User struct {
	ID          string `json:"id"`
	IAMUserID   string `json:"iam_user_id"`
	Email       string `json:"email"`
	Status      string `json:"status"`
}

// Authorization represents the authorization context in the auth response
type Authorization struct {
	Roles     []string `json:"roles"`
	Companies []string `json:"companies"`
}

// SessionResponse represents the response for session operations
type SessionResponse struct {
	User           User           `json:"user"`
	Authorization  Authorization  `json:"authorization"`
}