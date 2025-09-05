package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Options struct {
	Paths        []string // например: []string{"./", "./configs", "/etc/goshop"}
	Names        []string // например: []string{"config", "users"}
	Type         string
	EnvPrefix    string
	Defaults     map[string]any
	OptionalFile bool
}

func Load[T any](opts Options) (T, error) {
	var cfg T

	v := viper.New()

	cfgType := opts.Type
	if cfgType == "" {
		cfgType = "yaml"
	}
	v.SetConfigType(cfgType)

	for _, p := range opts.Paths {
		if p != "" {
			v.AddConfigPath(p)
		}
	}
	foundFile := false
	for _, name := range opts.Names {
		if name == "" {
			continue
		}
		v.SetConfigName(name)
		if err := v.ReadInConfig(); err == nil {
			foundFile = true
			break
		}
	}
	if !foundFile && !opts.OptionalFile {
		return cfg, fmt.Errorf("config file not found in paths %v for names %v", opts.Paths, opts.Names)
	}

	for k, val := range opts.Defaults {
		v.SetDefault(k, val)
	}

	if opts.EnvPrefix != "" {
		v.SetEnvPrefix(opts.EnvPrefix)
	}
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.Unmarshal(&cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config: %w", err)
	}

	return cfg, nil
}

func Require(cond bool, msg string) error {
	if !cond {
		return errors.New(msg)
	}
	return nil
}
