package jwtauth

import (
	"errors"
	"github.com/google/uuid"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

type Claims struct {
	UserID string `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func New(cfg Config) *Manager {
	return &Manager{
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
	}
}

func (m *Manager) GeneratePair(userID, email string) (access string, refresh string, refreshJTI uuid.UUID, err error) {
	jti := uuid.New()
	refresh, err = m.generateRefreshWithJTI(userID, email, jti)
	if err != nil {
		return "", "", uuid.Nil, err
	}
	access, err = m.generateAccess(userID, email)
	if err != nil {
		return "", "", uuid.Nil, err
	}
	return access, refresh, jti, nil
}

func (m *Manager) GeneratePairWithJTI(userID, email string, refreshJTI uuid.UUID) (access string, refresh string, err error) {
	refresh, err = m.generateRefreshWithJTI(userID, email, refreshJTI)
	if err != nil {
		return "", "", err
	}
	access, err = m.generateAccess(userID, email)
	return access, refresh, err
}

func (m *Manager) generateAccess(userID, email string) (string, error) {
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
	return jwt.NewWithClaims(jwt.SigningMethodHS256, ac).SignedString(m.secret)
}

func (m *Manager) generateRefreshWithJTI(userID, email string, jti uuid.UUID) (string, error) {
	now := time.Now().UTC()
	rc := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			ID:        jti.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, rc).SignedString(m.secret)
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
