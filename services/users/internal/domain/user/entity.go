package user

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash []byte
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var (
	ErrInvalidEmail = errors.New("invalid email")
)

func NormalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func ValidateEmail(s string) error {
	email := NormalizeEmail(s)
	if email == "" || len(email) > 320 {
		return ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return ErrInvalidEmail
	}
	if strings.ContainsAny(email, " \t\r\n") {
		return ErrInvalidEmail
	}
	return nil
}
