package config

import (
	"errors"
	"fmt"
	"strings"

	cfg "goshop/pkg/config"

	"github.com/spf13/viper"
)

type Users struct {
	AppName   string        `mapstructure:"app_name"`
	HTTP      cfg.HTTP      `mapstructure:"http"`
	Postgres  cfg.Postgres  `mapstructure:"postgres"`
	Redis     cfg.Redis     `mapstructure:"redis"`
	Logger    cfg.Logger    `mapstructure:"logger"`
	Telemetry cfg.Telemetry `mapstructure:"telemetry"`
	Auth      Auth          `mapstructure:"auth"`
}

type Auth struct {
	SigningKey string `mapstructure:"signing_key"`
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
	if u.Auth.SigningKey == "" {
		return errors.New("auth.signing_key is required (no defaults for secrets)")
	}
	return nil
}

func Load() (*Users, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("USERS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigFile("./services/users/config/defaults.yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read defaults.yaml: %w", err)
	}

	ov := viper.New()
	ov.SetConfigType("yaml")
	ov.AddConfigPath("./")
	ov.AddConfigPath("./configs")
	ov.AddConfigPath("/etc/goshop")
	for _, name := range []string{"users", "config"} {
		ov.SetConfigName(name)
		if err := ov.ReadInConfig(); err == nil {
			if err := v.MergeConfigMap(ov.AllSettings()); err != nil {
				return nil, fmt.Errorf("merge %s.yaml: %w", name, err)
			}
			break
		}
	}

	var cfg Users
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal users config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid users config: %w", err)
	}
	cfg.Logger.AppName = cfg.AppName
	return &cfg, nil
}

// NewConfig (defaults.yaml → users.yaml → ENV USERS_*)
func NewConfig() *Users {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}
