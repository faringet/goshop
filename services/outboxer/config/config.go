package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	cfg "goshop/pkg/config"

	"github.com/spf13/viper"
)

type OutboxerConfig struct {
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

func (o *OutboxerConfig) Validate() error {
	if o.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := o.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}

func Load() (*OutboxerConfig, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ORDERS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigFile("./services/outboxer/config/defaults.yaml")
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read defaults.yaml: %w", err)
	}

	ov := viper.New()
	ov.SetConfigType("yaml")
	ov.AddConfigPath("./")
	ov.AddConfigPath("./configs")
	ov.AddConfigPath("/etc/goshop")
	for _, name := range []string{"orders", "config"} {
		ov.SetConfigName(name)
		if err := ov.ReadInConfig(); err == nil {
			if err := v.MergeConfigMap(ov.AllSettings()); err != nil {
				return nil, fmt.Errorf("merge %s.yaml: %w", name, err)
			}
			break
		}
	}

	var c OutboxerConfig
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if len(c.Kafka.Brokers) == 0 {
		return nil, errors.New("kafka.brokers is required")
	}
	if err := c.Postgres.Validate(); err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	c.Logger.AppName = c.AppName
	return &c, nil
}

func NewConfig() *OutboxerConfig {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}
