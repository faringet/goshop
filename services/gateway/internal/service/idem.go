package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	idemKeyPrefix = "idem:"
	idemBusyVal   = "in-progress"
)

type IdemStore struct {
	rdb *redis.Client
}

func NewIdemStore(rdb *redis.Client) *IdemStore { return &IdemStore{rdb: rdb} }

func (s *IdemStore) key(k string) string { return idemKeyPrefix + k }

// TryBegin пытается занять ключ на время busyTTL.
// true ⇒ мы первые, можно делать работу.
// false ⇒ ключ уже есть.
func (s *IdemStore) TryBegin(ctx context.Context, k string, busyTTL time.Duration) (bool, error) {
	if s.rdb == nil {
		return true, nil
	} // no-redis fallback
	ok, err := s.rdb.SetNX(ctx, s.key(k), idemBusyVal, busyTTL).Result()
	return ok, err
}

// Load проверяет значение ключа (busy или готовый ответ).
func (s *IdemStore) Load(ctx context.Context, k string) (string, error) {
	if s.rdb == nil {
		return "", nil
	}
	return s.rdb.Get(ctx, s.key(k)).Result()
}

// Commit сохраняет JSON-ответ на TTL.
func (s *IdemStore) Commit(ctx context.Context, k string, resp any, ttl time.Duration) error {
	if s.rdb == nil {
		return nil
	}
	b, _ := json.Marshal(resp)
	return s.rdb.Set(ctx, s.key(k), string(b), ttl).Err()
}
