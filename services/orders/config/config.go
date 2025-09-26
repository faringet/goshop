package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	cfg "goshop/pkg/config"

	"github.com/spf13/viper"
)

type Orders struct {
	AppName  string       `mapstructure:"app_name"`
	HTTP     cfg.HTTP     `mapstructure:"http"`
	Postgres cfg.Postgres `mapstructure:"postgres"`
	Logger   cfg.Logger   `mapstructure:"logger"`
	Kafka    cfg.Kafka    `mapstructure:"kafka"`
	Consumer Consumer     `mapstructure:"consumer"`
}

type Consumer struct {
	Group            string        `mapstructure:"group"`
	Topic            string        `mapstructure:"topic"`
	SessionTimeout   time.Duration `mapstructure:"session_timeout"`
	RebalanceTimeout time.Duration `mapstructure:"rebalance_timeout"`
}

func (o *Orders) Validate() error {
	if o.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := o.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	return nil
}

func Load() (*Orders, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("ORDERS") // <-- свой префикс
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetConfigFile("./services/orders/config/defaults.yaml")
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

	var cfg Orders
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal orders config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid orders config: %w", err)
	}
	cfg.Logger.AppName = cfg.AppName
	return &cfg, nil
}

func NewConfig() *Orders {
	c, err := Load()
	if err != nil {
		panic(err)
	}
	return c
}
