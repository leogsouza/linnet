package service

import (
	"errors"
)

var (
	// ErrUserNotFound used when the user wasn't found on the db.
	ErrUserNotFound = errors.New("user not found")
)

// User model
type User struct {
	ID       int64  `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}
