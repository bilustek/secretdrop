package model

import (
	"errors"
	"net/http"
)

// HTTP status code aliases for use in AppError.
const (
	StatusForbidden           = http.StatusForbidden
	StatusNotFound            = http.StatusNotFound
	StatusGone                = http.StatusGone
	StatusUnprocessableEntity = http.StatusUnprocessableEntity
	StatusInternalServerError = http.StatusInternalServerError
)

// AppError represents a structured API error with type and HTTP status code.
type AppError struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	return e.Message
}

// Sentinel errors for the secret lifecycle.
var (
	ErrNotFound      = errors.New("not found")
	ErrExpired       = errors.New("secret has expired")
	ErrAlreadyViewed = errors.New("secret has already been viewed")
	ErrEmailMismatch = errors.New("email does not match")
	ErrDecryptFailed = errors.New("decryption failed")
	ErrTextTooLong   = errors.New("text exceeds maximum length")
	ErrTooManyRecips = errors.New("too many recipients")
	ErrInvalidEmail  = errors.New("invalid email address")
	ErrLimitReached  = errors.New("secret creation limit reached")
)
