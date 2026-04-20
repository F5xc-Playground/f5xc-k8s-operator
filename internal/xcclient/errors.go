package xcclient

import (
	"errors"
	"fmt"
)

// Sentinel errors for well-known HTTP error categories.
var (
	ErrNotFound    = errors.New("not found")
	ErrConflict    = errors.New("conflict")
	ErrRateLimited = errors.New("rate limited")
	ErrServerError = errors.New("server error")
	ErrAuth        = errors.New("authentication/authorization error")
)

// APIError carries the HTTP status code, endpoint, and response body message
// alongside a sentinel error so callers can use errors.Is / errors.As.
type APIError struct {
	StatusCode int
	Endpoint   string
	Message    string
	Err        error
}

// Error implements the error interface. The returned string includes the status
// code, endpoint, and message so it is human-readable without unwrapping.
func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d on %s: %s", e.StatusCode, e.Endpoint, e.Message)
}

// Unwrap returns the inner sentinel error, enabling errors.Is / errors.As
// traversal through the error chain.
func (e *APIError) Unwrap() error {
	return e.Err
}

// StatusToError maps an HTTP status code to the appropriate sentinel error
// wrapped inside an *APIError. For status codes that do not map to a known
// sentinel, a plain error is returned.
func StatusToError(statusCode int, endpoint, body string) error {
	var sentinel error
	switch {
	case statusCode == 404:
		sentinel = ErrNotFound
	case statusCode == 401 || statusCode == 403:
		sentinel = ErrAuth
	case statusCode == 409:
		sentinel = ErrConflict
	case statusCode == 429:
		sentinel = ErrRateLimited
	case statusCode >= 500:
		sentinel = ErrServerError
	default:
		return fmt.Errorf("unexpected API error %d on %s: %s", statusCode, endpoint, body)
	}

	return &APIError{
		StatusCode: statusCode,
		Endpoint:   endpoint,
		Message:    body,
		Err:        sentinel,
	}
}
