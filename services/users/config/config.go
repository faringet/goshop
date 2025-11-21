package config

import (
	"errors"
	"fmt"
	cfg "goshop/pkg/config"
)

type Users struct {
	AppName   string        `mapstructure:"app_name"`
	HTTP      cfg.HTTP      `mapstructure:"http"`
	Postgres  cfg.Postgres  `mapstructure:"postgres"`
	Redis     cfg.Redis     `mapstructure:"redis"`
	Logger    cfg.Logger    `mapstructure:"logger"`
	Telemetry cfg.Telemetry `mapstructure:"telemetry"`
	JWT       cfg.JWT       `mapstructure:"jwt"`
}

func (u *Users) Validate() error {
	if u.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := u.HTTP.Validate(); err != nil {
		return fmt.Errorf("http: %w", err)
	}
	if err := u.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := u.Redis.Validate(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	if err := u.Telemetry.Validate(); err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	if u.JWT.Secret == "" {
		return errors.New("jwt.secret is required (no defaults for secrets)")
	}
	return nil
}

func (u Users) Redact() any {
	u.JWT.Secret = "***"
	u.Postgres.Password = "***"
	return u
}

// New — грузим конфиг по схеме: файлы -> ENV (с префиксом USERS_)
func New() *Users {
	c := cfg.MustLoad[Users](cfg.Options{
		Paths:         []string{"./config", "./services/users/config", "./configs", "/etc/goshop"},
		Names:         []string{"defaults", "users", "config"},
		Type:          "yaml",
		EnvPrefix:     "USERS",
		OptionalFiles: true, // false - требовать хотя бы один файл
	})
	c.Logger.AppName = c.AppName
	return c
}
