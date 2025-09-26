package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	cfg "goshop/pkg/config"

	"github.com/spf13/viper"
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

func Load() (*Payments, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("USERS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigFile("./services/payments/config/defaults.yaml")
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

	var cfg Payments
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := cfg.Postgres.Validate(); err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}
	if len(cfg.Kafka.Brokers) == 0 {
		return nil, errors.New("kafka.brokers is required")
	}
	if cfg.Consumer.Group == "" || cfg.Consumer.Topic == "" {
		return nil, errors.New("consumer.group and consumer.topic are required")
	}
	cfg.Logger.AppName = cfg.AppName
	return &cfg, nil
}

func NewConfig() *Payments {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}
