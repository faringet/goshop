package jwtauth

import (
	"errors"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

type Config struct {
	Secret     string
	Issuer     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type Manager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// Claims — наши клеймы + стандартные.
type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// New создаёт менеджер.
func New(cfg Config) *Manager {
	return &Manager{
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}
}

func (m *Manager) GeneratePair(userID, email string) (access string, refresh string, err error) {
	now := time.Now().UTC()

	ac := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	access, err = jwt.NewWithClaims(jwt.SigningMethodHS256, ac).SignedString(m.secret)
	if err != nil {
		return "", "", err
	}

	rc := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
		},
	}
	refresh, err = jwt.NewWithClaims(jwt.SigningMethodHS256, rc).SignedString(m.secret)
	if err != nil {
		return "", "", err
	}

	return access, refresh, nil
}

func (m *Manager) ParseAndVerify(tokenStr string) (*Claims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}))
	tok, err := parser.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid token")
	}
	if m.issuer != "" && claims.Issuer != "" && claims.Issuer != m.issuer {
		return nil, errors.New("invalid issuer")
	}
	return claims, nil
}

func (m *Manager) ExpiresIn() int64 {
	return int64(m.accessTTL / time.Second)
}
