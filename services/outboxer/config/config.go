package config

import (
	"errors"
	"fmt"
	"time"

	cfg "goshop/pkg/config"
)

type Outboxer struct {
	AppName  string       `mapstructure:"app_name"`
	Postgres cfg.Postgres `mapstructure:"postgres"`
	Logger   cfg.Logger   `mapstructure:"logger"`
	Kafka    cfg.Kafka    `mapstructure:"kafka"`
	Worker   Worker       `mapstructure:"worker"`
	Workers  []Worker     `mapstructure:"workers"`
}

type Worker struct {
	OutboxTable    string        `mapstructure:"outbox_table"`
	BatchSize      int           `mapstructure:"batch_size"`
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	ProduceTimeout time.Duration `mapstructure:"produce_timeout"`
	MaxRetries     int           `mapstructure:"max_retries"`
	BackoffBaseMS  int           `mapstructure:"backoff_base_ms"`
}

func (c *Outboxer) Validate() error {
	if c.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := c.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if len(c.Workers) == 0 && c.Worker.OutboxTable == "" {
		return errors.New("either workers[] or worker must be provided")
	}
	return nil
}

func (c *Outboxer) AllWorkers() []Worker {
	if len(c.Workers) > 0 {
		return c.Workers
	}
	if c.Worker.OutboxTable != "" {
		return []Worker{c.Worker}
	}
	return nil
}

// New — грузим конфиг по схеме: файлы -> ENV (с префиксом OUTBOXER_)
func New() *Outboxer {
	c := cfg.MustLoad[Outboxer](cfg.Options{
		Paths:         []string{"./services/outboxer/config", "./configs", "/etc/goshop"},
		Names:         []string{"defaults", "outboxer", "config"},
		Type:          "yaml",
		EnvPrefix:     "OUTBOXER",
		OptionalFiles: true, // false - требовать хотя бы один файл
	})
	c.Logger.AppName = c.AppName
	return c
}
