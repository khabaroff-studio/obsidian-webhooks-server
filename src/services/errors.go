package services

import "errors"

// Sentinel errors for explicit error handling
// These errors allow callers to distinguish between different failure modes
// using errors.Is() instead of string matching

var (
	// ErrKeyNotFound indicates the requested key does not exist
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidKeyType indicates an invalid key type was provided
	ErrInvalidKeyType = errors.New("invalid key type")

	// ErrUserNotFound indicates the user does not exist
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidCredentials indicates authentication failed
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrTransactionFailed indicates a database transaction failed
	ErrTransactionFailed = errors.New("transaction failed")
)
