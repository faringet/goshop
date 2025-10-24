package config

import (
	"errors"
	"fmt"
	"time"

	cfg "goshop/pkg/config"
)

type OpsAssistant struct {
	AppName  string       `mapstructure:"app_name"`
	Logger   cfg.Logger   `mapstructure:"logger"`
	Postgres cfg.Postgres `mapstructure:"postgres"`

	GRPC struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"grpc"`

	Ollama struct {
		BaseURL string        `mapstructure:"base_url"`
		Model   string        `mapstructure:"model"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"ollama"`
}

func (c *OpsAssistant) Validate() error {
	if c.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := c.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}

	if c.GRPC.Addr == "" {
		return errors.New("grpc.addr is required")
	}

	if c.Ollama.BaseURL == "" {
		return errors.New("ollama.base_url is required")
	}
	if c.Ollama.Model == "" {
		return errors.New("ollama.model is required")
	}
	if c.Ollama.Timeout <= 0 {
		c.Ollama.Timeout = 5 * time.Second
	}

	return nil
}

func New() *OpsAssistant {
	c := cfg.MustLoad[OpsAssistant](cfg.Options{
		Paths:         []string{"./services/opsassistant/config", "./configs", "/etc/goshop"},
		Names:         []string{"defaults", "opsassistant", "config"},
		Type:          "yaml",
		EnvPrefix:     "OPSASSISTANT",
		OptionalFiles: true,
	})
	c.Logger.AppName = c.AppName
	return c
}
