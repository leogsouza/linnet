package service

import (
	"errors"
	"regexp"
)

var (
	rxEmail = regexp.MustCompile("^[^\\s@]+@[^\\s@]+\\.[^\\s@]+$")
	// ErrUserNotFound used when the user wasn't found on the db.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidEmail used when the email is invalid.
	ErrInvalidEmail = errors.New("invalid email")
)

// User model
type User struct {
	ID       int64  `json:"id,omitempty"`
	Username string `json:"username,omitempty"`
}
