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
	Worker   struct {
		BatchSize      int           `mapstructure:"batch_size"`
		PollInterval   time.Duration `mapstructure:"poll_interval"`
		ProduceTimeout time.Duration `mapstructure:"produce_timeout"`
		MaxRetries     int           `mapstructure:"max_retries"`
		BackoffBaseMS  int           `mapstructure:"backoff_base_ms"`
	} `mapstructure:"worker"`
}

func (o *Outboxer) Validate() error {
	if o.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := o.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
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
