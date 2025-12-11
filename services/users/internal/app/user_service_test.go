package app_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	app "goshop/services/users/internal/app"
	domain "goshop/services/users/internal/domain/user"
)

type mockUserRepo struct {
	createFn     func(ctx context.Context, email string, passwordHash []byte) (domain.User, error)
	getByEmailFn func(ctx context.Context, email string) (domain.User, error)

	createCalled     int
	getByEmailCalled int

	lastCreateEmail     string
	lastCreateHash      []byte
	lastGetByEmailEmail string
}

var _ app.UserRepository = (*mockUserRepo)(nil)

func (m *mockUserRepo) Create(ctx context.Context, email string, passwordHash []byte) (domain.User, error) {
	m.createCalled++
	m.lastCreateEmail = email
	m.lastCreateHash = append([]byte(nil), passwordHash...)

	if m.createFn != nil {
		return m.createFn(ctx, email, passwordHash)
	}
	return domain.User{}, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	m.getByEmailCalled++
	m.lastGetByEmailEmail = email

	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return domain.User{}, nil
}

func TestRegister_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	expected := domain.User{
		Email: "user@example.com",
	}

	repo := &mockUserRepo{
		createFn: func(ctx context.Context, email string, hash []byte) (domain.User, error) {
			return expected, nil
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	u, err := svc.Register(ctx, "  User@Example.COM  ", "StrongPass123!")
	if err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	if repo.createCalled != 1 {
		t.Fatalf("expected Create to be called once, got %d", repo.createCalled)
	}

	if repo.lastCreateEmail != "user@example.com" {
		t.Fatalf("Create called with email %q, want %q", repo.lastCreateEmail, "user@example.com")
	}

	if len(repo.lastCreateHash) == 0 {
		t.Fatalf("password hash is empty")
	}
	if bytes.Equal(repo.lastCreateHash, []byte("StrongPass123!")) {
		t.Fatalf("password hash equals the plain password")
	}

	if err := bcrypt.CompareHashAndPassword(repo.lastCreateHash, []byte("StrongPass123!")); err != nil {
		t.Fatalf("stored hash does not match password: %v", err)
	}
	cost, err := bcrypt.Cost(repo.lastCreateHash)
	if err != nil {
		t.Fatalf("bcrypt.Cost failed: %v", err)
	}
	if cost != bcrypt.MinCost {
		t.Fatalf("unexpected bcrypt cost: got %d, want %d (MinCost)", cost, bcrypt.MinCost)
	}

	if u.Email != expected.Email {
		t.Fatalf("Service.Register returned user with email %q, want %q", u.Email, expected.Email)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	repo := &mockUserRepo{
		createFn: func(ctx context.Context, email string, hash []byte) (domain.User, error) {
			t.Fatalf("Create should not be called for invalid email")
			return domain.User{}, nil
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	_, err := svc.Register(ctx, "not-an-email", "StrongPass123!")
	if !errors.Is(err, domain.ErrInvalidEmail) {
		t.Fatalf("Register error = %v, want %v", err, domain.ErrInvalidEmail)
	}

	if repo.createCalled != 0 {
		t.Fatalf("Create called %d times, want 0", repo.createCalled)
	}
}

func TestRegister_WeakPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	repo := &mockUserRepo{
		createFn: func(ctx context.Context, email string, hash []byte) (domain.User, error) {
			t.Fatalf("Create should not be called for weak password")
			return domain.User{}, nil
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	_, err := svc.Register(ctx, "user@example.com", "short")
	if !errors.Is(err, app.ErrWeakPassword) {
		t.Fatalf("Register error = %v, want %v", err, app.ErrWeakPassword)
	}

	if repo.createCalled != 0 {
		t.Fatalf("Create called %d times, want 0", repo.createCalled)
	}
}

func TestRegister_BcryptError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	repo := &mockUserRepo{
		createFn: func(ctx context.Context, email string, hash []byte) (domain.User, error) {
			t.Fatalf("Create should not be called when bcrypt fails")
			return domain.User{}, nil
		},
	}

	svc := app.NewService(repo, 100) // > bcrypt.MaxCost

	_, err := svc.Register(ctx, "user@example.com", "StrongPass123!")
	if err == nil {
		t.Fatalf("Register error = nil, want non-nil due to bcrypt error")
	}

	if repo.createCalled != 0 {
		t.Fatalf("Create called %d times, want 0", repo.createCalled)
	}
}

func TestAuthenticate_Success(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	password := "StrongPass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword failed: %v", err)
	}

	expected := domain.User{
		Email:        "user@example.com",
		PasswordHash: hash,
	}

	repo := &mockUserRepo{
		getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
			return expected, nil
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	u, err := svc.Authenticate(ctx, "  User@Example.COM  ", password)
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}

	if repo.getByEmailCalled != 1 {
		t.Fatalf("GetByEmail called %d times, want 1", repo.getByEmailCalled)
	}

	if repo.lastGetByEmailEmail != "user@example.com" {
		t.Fatalf("GetByEmail called with email %q, want %q", repo.lastGetByEmailEmail, "user@example.com")
	}

	if u.Email != expected.Email {
		t.Fatalf("Authenticate returned user with email %q, want %q", u.Email, expected.Email)
	}
	if !bytes.Equal(u.PasswordHash, expected.PasswordHash) {
		t.Fatalf("Authenticate returned user with different password hash")
	}
}

func TestAuthenticate_InvalidEmail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	repo := &mockUserRepo{
		getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
			t.Fatalf("GetByEmail should not be called for invalid email")
			return domain.User{}, nil
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	_, err := svc.Authenticate(ctx, "not-an-email", "whatever")
	if !errors.Is(err, app.ErrInvalidCredentials) {
		t.Fatalf("Authenticate error = %v, want %v", err, app.ErrInvalidCredentials)
	}

	if repo.getByEmailCalled != 0 {
		t.Fatalf("GetByEmail called %d times, want 0", repo.getByEmailCalled)
	}
}

func TestAuthenticate_RepoError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	repo := &mockUserRepo{
		getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
			return domain.User{}, errors.New("db down")
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	_, err := svc.Authenticate(ctx, "user@example.com", "StrongPass123!")
	if !errors.Is(err, app.ErrInvalidCredentials) {
		t.Fatalf("Authenticate error = %v, want %v", err, app.ErrInvalidCredentials)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	password := "StrongPass123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword failed: %v", err)
	}

	repo := &mockUserRepo{
		getByEmailFn: func(ctx context.Context, email string) (domain.User, error) {
			return domain.User{
				Email:        "user@example.com",
				PasswordHash: hash,
			}, nil
		},
	}

	svc := app.NewService(repo, bcrypt.MinCost)

	_, err = svc.Authenticate(ctx, "user@example.com", "WrongPassword!")
	if !errors.Is(err, app.ErrInvalidCredentials) {
		t.Fatalf("Authenticate error = %v, want %v", err, app.ErrInvalidCredentials)
	}
}
