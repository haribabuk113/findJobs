package profiles

import "fmt"

// Domain errors
var (
	ErrFirstNameRequired = &ValidationError{Field: "first_name", Message: "first name is required"}
	ErrLastNameRequired  = &ValidationError{Field: "last_name", Message: "last name is required"}
	ErrProfileNotFound   = &NotFoundError{Message: "profile not found"}
	ErrInvalidProfileID  = &ValidationError{Field: "id", Message: "invalid profile ID"}
)

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

// IsNotFound checks if an error is a NotFoundError
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// IsValidation checks if an error is a ValidationError
func IsValidation(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// NewValidationError creates a validation error with a formatted message
func NewValidationError(field, format string, args ...any) *ValidationError {
		return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}
