package handler

import "errors"

// Handler errors for request processing.
var (
	// ErrEmptyBody is returned when a request body is required but empty.
	ErrEmptyBody = errors.New("request body is empty")

	// ErrInvalidJSON is returned when the request body contains invalid JSON.
	ErrInvalidJSON = errors.New("invalid JSON in request body")

	// ErrUnauthorized is returned when authentication is required but missing or invalid.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden is returned when the user lacks permission for the requested action.
	ErrForbidden = errors.New("forbidden")

	// ErrNotFound is returned when the requested resource does not exist.
	ErrNotFound = errors.New("resource not found")
)
