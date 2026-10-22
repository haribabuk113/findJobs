package errors

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return e.Message
}

func IsNotFound(err error) bool {
	var ae *Error
	return errors.As(err, &ae) && ae.Status == http.StatusNotFound
}
