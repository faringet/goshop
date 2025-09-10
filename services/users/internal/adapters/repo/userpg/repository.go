package userpg

import (
	"context"
	"errors"

	"goshop/services/users/internal/app"
	domain "goshop/services/users/internal/domain/user"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("users: not found")
	ErrEmailTaken = errors.New("users: email already taken")
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepo(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ app.UserRepository = (*Repository)(nil)

func (r *Repository) Create(ctx context.Context, email string, passwordHash []byte) (domain.User, error) {
	email = domain.NormalizeEmail(email)
	if err := domain.ValidateEmail(email); err != nil {
		return domain.User{}, err
	}

	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at, updated_at
	`
	var u domain.User
	err := r.db.QueryRow(ctx, q, email, passwordHash).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrEmailTaken
		}
		return domain.User{}, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	return u, nil
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	email = domain.NormalizeEmail(email)

	const q = `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u domain.User
	err := r.db.QueryRow(ctx, q, email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, err
	}
	u.CreatedAt = u.CreatedAt.UTC()
	u.UpdatedAt = u.UpdatedAt.UTC()
	return u, nil
}
