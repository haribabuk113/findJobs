package auth

import "fmt"

// Domain errors for auth
var (
	ErrCodeRequired = &ValidationError{Field: "code", Message: "code is required"}
	ErrUserNotFound = &NotFoundError{Message: "user not found"}
	// ErrUnauthorized: no session, expired session, or IAM rejected the
	// code/refresh token (expired, reused, account suspended).
	ErrUnauthorized = &UnauthorizedError{Message: "not authenticated"}
)

// UnauthorizedError represents a missing or rejected authentication
type UnauthorizedError struct {
	Message string
}

// Error implements the error interface
func (e *UnauthorizedError) Error() string {
	return e.Message
}

// IsUnauthorized checks if an error is an UnauthorizedError
func IsUnauthorized(err error) bool {
	_, ok := err.(*UnauthorizedError)
	return ok
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return e.Message
}

// NotFoundError represents a "not found" error
type NotFoundError struct {
	Message string
}

// Error implements the error interface
func (e *NotFoundError) Error() string {
	return e.Message
}

// InternalError represents an internal server error
type InternalError struct {
	Message string
}

// Error implements the error interface
func (e *InternalError) Error() string {
	return e.Message
}

// IsValidation checks if an error is a ValidationError
func IsValidation(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// IsInternal checks if an error is an InternalError
func IsInternal(err error) bool {
	_, ok := err.(*InternalError)
	return ok
}

// NewValidationError creates a validation error with a formatted message
func NewValidationError(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// NewInternalError creates an internal error with a formatted message
func NewInternalError(format string, args ...any) *InternalError {
	return &InternalError{Message: fmt.Sprintf(format, args...)}
}
