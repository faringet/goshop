package config

import (
	"errors"
	"fmt"
	"time"

	cfg "goshop/pkg/config"
)

type Orders struct {
	AppName  string       `mapstructure:"app_name"`
	HTTP     cfg.HTTP     `mapstructure:"http"`
	Postgres cfg.Postgres `mapstructure:"postgres"`
	Logger   cfg.Logger   `mapstructure:"logger"`
	Kafka    cfg.Kafka    `mapstructure:"kafka"`
	JWT      cfg.JWT      `mapstructure:"jwt"`
	Redis    Redis        `mapstructure:"redis"`
	Consumer Consumer     `mapstructure:"consumer"`
	GRPC     GRPC         `mapstructure:"grpc"`
}

type Consumer struct {
	Group            string        `mapstructure:"group"`
	Topic            string        `mapstructure:"topic"`
	SessionTimeout   time.Duration `mapstructure:"session_timeout"`
	RebalanceTimeout time.Duration `mapstructure:"rebalance_timeout"`
}

type Redis struct {
	Addr      string        `mapstructure:"addr"`
	Password  string        `mapstructure:"password"`
	DB        int           `mapstructure:"db"`
	TTLStatus time.Duration `mapstructure:"ttl_status"`
}

type GRPC struct {
	Addr string `mapstructure:"addr"`
}

func (o *Orders) Validate() error {
	if o.AppName == "" {
		return errors.New("app_name is required")
	}
	if err := o.Postgres.Validate(); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if o.JWT.Secret == "" {
		return errors.New("jwt.secret is required (for token verification)")
	}
	if o.Redis.Addr == "" {
		o.Redis.Addr = "localhost:6379"
	}
	if o.Redis.TTLStatus <= 0 {
		o.Redis.TTLStatus = 24 * time.Hour
	}
	return nil
}

// New — грузим конфиг по схеме: файлы -> ENV (с префиксом ORDERS_)
func New() *Orders {
	c := cfg.MustLoad[Orders](cfg.Options{
		Paths:         []string{"./config", "./services/orders/config", "./configs", "/etc/goshop"},
		Names:         []string{"defaults", "orders", "config"},
		Type:          "yaml",
		EnvPrefix:     "ORDERS",
		OptionalFiles: true, // false - требовать хотя бы один файл
	})
	c.Logger.AppName = c.AppName
	return c
}
