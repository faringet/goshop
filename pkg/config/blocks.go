package config

import (
	"errors"
	"fmt"
	"time"
)

type HTTP struct {
	Addr         string        `mapstructure:"addr"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

func (h *HTTP) Validate() error {
	if h == nil {
		return errors.New("http config is nil")
	}
	if h.Addr == "" {
		return errors.New("http.addr is required")
	}
	return nil
}

type GRPC struct {
	Addr           string        `mapstructure:"addr"`
	MaxRecvMsgSize int           `mapstructure:"max_recv_msg_size"`
	MaxSendMsgSize int           `mapstructure:"max_send_msg_size"`
	KeepaliveTime  time.Duration `mapstructure:"keepalive_time"`
	KeepaliveTO    time.Duration `mapstructure:"keepalive_timeout"`
}

func (g *GRPC) Validate() error {
	if g == nil {
		return errors.New("grpc config is nil")
	}
	if g.Addr == "" {
		return errors.New("grpc.addr is required")
	}
	return nil
}

type Postgres struct {
	Host       string        `mapstructure:"host"`
	Port       int           `mapstructure:"port"`
	User       string        `mapstructure:"user"`
	Password   string        `mapstructure:"password"`
	DBName     string        `mapstructure:"dbname"`
	SSLMode    string        `mapstructure:"sslmode"`
	MaxConns   int32         `mapstructure:"max_conns"`
	MinConns   int32         `mapstructure:"min_conns"`
	ConnLife   time.Duration `mapstructure:"conn_life"`
	HealthPing time.Duration `mapstructure:"health_ping"`
}

func (p *Postgres) DSN() string {
	ssl := p.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DBName, ssl,
	)
}

func (p *Postgres) Validate() error {
	if p.Host == "" {
		return errors.New("postgres.host is required")
	}
	if p.Port == 0 {
		return errors.New("postgres.port is required")
	}
	if p.User == "" {
		return errors.New("postgres.user is required")
	}
	if p.DBName == "" {
		return errors.New("postgres.dbname is required")
	}
	return nil
}

type Redis struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

func (r *Redis) Validate() error {
	if r == nil {
		return errors.New("redis config is nil")
	}
	if r.Addr == "" {
		return errors.New("redis.addr is required")
	}
	return nil
}

type Kafka struct {
	Brokers []string `mapstructure:"brokers"`
}

func (k *Kafka) Validate() error {
	if k == nil {
		return errors.New("kafka config is nil")
	}
	if len(k.Brokers) == 0 {
		return errors.New("kafka.brokers is required")
	}
	return nil
}

type JWT struct {
	Secret          string        `mapstructure:"secret"`
	Issuer          string        `mapstructure:"issuer"`
	AccessTTL       time.Duration `mapstructure:"access_ttl"`
	RefreshTTL      time.Duration `mapstructure:"refresh_ttl"`
	AccessAudience  string        `mapstructure:"access_audience"`
	RefreshAudience string        `mapstructure:"refresh_audience"`
}

type Logger struct {
	Level   string `mapstructure:"level"`
	JSON    bool   `mapstructure:"json"`
	AppName string `mapstructure:"app_name"`
}

type Telemetry struct {
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	SampleRatio  float64 `mapstructure:"sample_ratio"`
}

func (t *Telemetry) Validate() error {
	return nil
}
