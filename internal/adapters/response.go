package adapters

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Response is used for static shape json return
type Response struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Error   any    `json:"error"`
	Data    any    `json:"data"`
}

func validationMessage(err error) error {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		parts := make([]string, 0, len(ve))
		for _, fe := range ve {
			parts = append(parts, fmt.Sprintf("%s: failed '%s' validation", fe.Field(), fe.Tag()))
		}
		return errors.New(strings.Join(parts, "; "))
	}
	return fmt.Errorf("invalid request: %w", err)
}

// BuildResponse method is to inject data value to dynamic success response
func BuildResponse(message string, data any, err any) Response {
	var errmess any = nil
	if err != nil {
		errmess = err
	}
	res := Response{
		Status:  true,
		Message: message,
		Error:   errmess,
		Data:    data,
	}
	return res
}

// BuildErrorResponse method is to inject data value to dynamic failed response
func BuildErrorResponse(message string, data any, err error) Response {
	res := Response{
		Status:  false,
		Message: message,
		Error:   validationMessage(err).Error(),
		Data:    data,
	}
	return res
}
