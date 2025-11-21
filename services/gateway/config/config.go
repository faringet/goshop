package config

import (
	"errors"
	"fmt"
	"time"

	cfg "goshop/pkg/config"
)

type Gateway struct {
	AppName string     `mapstructure:"app_name"`
	Logger  cfg.Logger `mapstructure:"logger"`

	GRPC struct {
		Addr string `mapstructure:"addr"`
	} `mapstructure:"grpc"`

	OrdersGRPC struct {
		Addr    string        `mapstructure:"addr"`
		Timeout time.Duration `mapstructure:"timeout"`
	} `mapstructure:"orders_grpc"`

	Redis struct {
		Addr         string        `mapstructure:"addr"`
		Password     string        `mapstructure:"password"`
		DB           int           `mapstructure:"db"`
		DialTimeout  time.Duration `mapstructure:"dial_timeout"`
		ReadTimeout  time.Duration `mapstructure:"read_timeout"`
		WriteTimeout time.Duration `mapstructure:"write_timeout"`
	} `mapstructure:"redis"`
}

func (g *Gateway) Validate() error {
	if g.AppName == "" {
		return errors.New("app_name is required")
	}
	if g.GRPC.Addr == "" {
		return errors.New("grpc.addr is required")
	}
	if g.OrdersGRPC.Addr == "" {
		return errors.New("orders_grpc.addr is required")
	}
	if g.Redis.Addr == "" {
		return errors.New("redis.addr is required")
	}
	return nil
}

func New() *Gateway {
	c := cfg.MustLoad[Gateway](cfg.Options{
		Paths:         []string{"./config", "./services/gateway/config", "./configs", "/etc/goshop"},
		Names:         []string{"defaults", "gateway", "config"},
		Type:          "yaml",
		EnvPrefix:     "GATEWAY",
		OptionalFiles: true,
	})

	if c.OrdersGRPC.Timeout <= 0 {
		c.OrdersGRPC.Timeout = 3 * time.Second
	}

	if err := c.Validate(); err != nil {
		panic(fmt.Errorf("invalid gateway config: %w", err))
	}

	c.Logger.AppName = c.AppName
	return c
}
