package app

import (
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"

	domain "goshop/services/users/internal/domain/user"
)

var (
	ErrWeakPassword       = errors.New("users: weak password")
	ErrInvalidCredentials = errors.New("users: invalid credentials")
)

type UserRepository interface {
	Create(ctx context.Context, email string, passwordHash []byte) (domain.User, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
}

type Service struct {
	repo       UserRepository
	bcryptCost int
}

func NewService(repo UserRepository, bcryptCost int) *Service {
	if bcryptCost <= 0 {
		bcryptCost = 12
	}
	return &Service{repo: repo, bcryptCost: bcryptCost}
}

func (s *Service) Register(ctx context.Context, email, password string) (domain.User, error) {
	if err := domain.ValidateEmail(email); err != nil {
		return domain.User{}, err
	}
	if len(password) < 8 {
		return domain.User{}, ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return domain.User{}, err
	}
	u, err := s.repo.Create(ctx, domain.NormalizeEmail(email), hash)
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (domain.User, error) {
	if err := domain.ValidateEmail(email); err != nil {
		return domain.User{}, ErrInvalidCredentials
	}
	u, err := s.repo.GetByEmail(ctx, domain.NormalizeEmail(email))
	if err != nil {
		return domain.User{}, ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword(u.PasswordHash, []byte(password)) != nil {
		return domain.User{}, ErrInvalidCredentials
	}
	return u, nil
}
