package config

import (
	"errors"
	"fmt"
	"time"

	cfg "goshop/pkg/config"
)

type Payments struct {
	AppName  string       `mapstructure:"app_name"`
	Logger   cfg.Logger   `mapstructure:"logger"`
	Postgres cfg.Postgres `mapstructure:"postgres"`
	Kafka    cfg.Kafka    `mapstructure:"kafka"`
	Consumer struct {
		Group            string        `mapstructure:"group"`
		Topic            string        `mapstructure:"topic"`
		SessionTimeout   time.Duration `mapstructure:"session_timeout"`
		RebalanceTimeout time.Duration `mapstructure:"rebalance_timeout"`
	} `mapstructure:"consumer"`
	Outbox struct {
		Topic string `mapstructure:"topic"`
	} `mapstructure:"outbox"`
}

func (p *Payments) Validate() error {
	if p.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := p.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}

// New — грузим конфиг по схеме: файлы -> ENV (с префиксом PAYMENTS_)
func New() *Payments {
	c := cfg.MustLoad[Payments](cfg.Options{
		Paths:         []string{"./services/payments/config", "./configs", "/etc/goshop"},
		Names:         []string{"defaults", "payments", "config"},
		Type:          "yaml",
		EnvPrefix:     "PAYMENTS",
		OptionalFiles: true, // false - требовать хотя бы один файл
	})
	c.Logger.AppName = c.AppName
	return c
}
