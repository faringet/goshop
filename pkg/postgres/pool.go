package postgres

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	pcfg "goshop/pkg/config"
	"time"
)

const defaultPingTimeout = 3 * time.Second

func NewPool(ctx context.Context, c pcfg.Postgres) (*pgxpool.Pool, error) {
	conf, err := pgxpool.ParseConfig(c.DSN())
	if err != nil {
		return nil, fmt.Errorf("pgx parse config: %w", err)
	}

	if c.MaxConns > 0 {
		conf.MaxConns = c.MaxConns
	}
	if c.MinConns > 0 {
		conf.MinConns = c.MinConns
	}
	if c.ConnLife > 0 {
		conf.MaxConnLifetime = c.ConnLife
	}
	if c.HealthPing > 0 {
		conf.HealthCheckPeriod = c.HealthPing
	}

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("pgx new pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgx ping: %w", err)
	}

	return pool, nil
}
