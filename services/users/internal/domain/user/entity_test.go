package user_test

import (
	"strings"
	"testing"

	"goshop/services/users/internal/domain/user"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already_normalized", "user@example.com", "user@example.com"},
		{"trim_spaces", "   user@example.com   ", "user@example.com"},
		{"upper_to_lower", "User@Example.COM", "user@example.com"},
		{"trim_and_lower", "  User@Example.COM  ", "user@example.com"},
		{"empty", "", ""},
		{"only_spaces", "   ", ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := user.NormalizeEmail(tt.in)
			if got != tt.want {
				t.Fatalf("NormalizeEmail(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestValidateEmail_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{"simple", "user@example.com"},
		{"with_plus", "john.doe+tag@example.co.uk"},
		{"with_spaces_and_upper", "  User.Name+Tag@Example.COM  "},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if err := user.ValidateEmail(tt.in); err != nil {
				t.Fatalf("ValidateEmail(%q) returned error: %v", tt.in, err)
			}
		})
	}
}

func TestValidateEmail_Invalid(t *testing.T) {
	t.Parallel()

	longLocal := strings.Repeat("a", 310)
	tooLong := longLocal + "@example.com"

	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"only_spaces", "   "},
		{"no_at", "userexample.com"},
		{"space_inside", "user @example.com"},
		{"tab_inside", "user@\texample.com"},
		{"newline_inside", "user@\nexample.com"},
		{"too_long", tooLong},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := user.ValidateEmail(tt.in)
			if err == nil {
				t.Fatalf("ValidateEmail(%q) = nil, want error", tt.in)
			}
			if err != user.ErrInvalidEmail {
				t.Fatalf("ValidateEmail(%q) error = %v, want %v", tt.in, err, user.ErrInvalidEmail)
			}
		})
	}
}
