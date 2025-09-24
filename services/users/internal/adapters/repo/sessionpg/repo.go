package sessionpg

import (
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Repo {
	return &Repo{db: db}
}

var (
	ErrNotFound     = errors.New("session not found")
	ErrRevoked      = errors.New("session revoked")
	ErrRotated      = errors.New("session already rotated")
	ErrExpired      = errors.New("session expired")
	ErrRefreshReuse = errors.New("refresh reuse detected")
)

func (r *Repo) CreateSession(
	ctx context.Context,
	sessionID uuid.UUID,
	userID uuid.UUID,
	refreshHash []byte,
	expiresAt time.Time,
	userAgent string,
	ip net.IP,
) (uuid.UUID, error) {

	const q = `
        INSERT INTO sessions (id, user_id, refresh_hash, user_agent, ip, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id;
    `
	var id uuid.UUID
	if err := r.db.QueryRow(ctx, q, sessionID, userID, refreshHash, nullIfEmpty(userAgent), ip, expiresAt).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (r *Repo) RotateSession(
	ctx context.Context,
	oldID uuid.UUID,
	oldHash []byte,
	newID uuid.UUID,
	newHash []byte,
	newExpiresAt time.Time,
	userAgent string,
	ip net.IP,
) (uuid.UUID, error) {

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const sel = `
		SELECT user_id, refresh_hash, revoked_at, rotated_at, expires_at
		FROM sessions
		WHERE id = $1
		FOR UPDATE;
	`
	var (
		userID    uuid.UUID
		dbHash    []byte
		revokedAt *time.Time
		rotatedAt *time.Time
		expiresAt time.Time
	)
	if err := tx.QueryRow(ctx, sel, oldID).Scan(&userID, &dbHash, &revokedAt, &rotatedAt, &expiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	if revokedAt != nil {
		return uuid.Nil, ErrRevoked
	}
	if rotatedAt != nil {
		return uuid.Nil, ErrRotated
	}
	if time.Now().After(expiresAt) {
		return uuid.Nil, ErrExpired
	}
	if subtle.ConstantTimeCompare(dbHash, oldHash) != 1 {
		const revoke = `UPDATE sessions SET revoked_at = now() WHERE id = $1;`
		if _, uerr := tx.Exec(ctx, revoke, oldID); uerr != nil {
			return uuid.Nil, uerr
		}
		return uuid.Nil, ErrRefreshReuse
	}

	const markRotated = `UPDATE sessions SET rotated_at = now() WHERE id = $1;`
	if _, err := tx.Exec(ctx, markRotated, oldID); err != nil {
		return uuid.Nil, err
	}

	const ins = `
		INSERT INTO sessions (id, user_id, refresh_hash, user_agent, ip, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id;
	`
	var ret uuid.UUID
	if err := tx.QueryRow(ctx, ins, newID, userID, newHash, nullIfEmpty(userAgent), ip, newExpiresAt).Scan(&ret); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return ret, nil
}

func (r *Repo) RevokeSession(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;`
	ct, err := r.db.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	_ = ct
	return nil
}

func (r *Repo) RevokeAll(ctx context.Context, userID uuid.UUID) (int64, error) {
	const q = `
		UPDATE sessions
		SET revoked_at = now()
		WHERE user_id = $1 AND revoked_at IS NULL;
	`
	ct, err := r.db.Exec(ctx, q, userID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
