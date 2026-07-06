package config

import "github.com/wangweihong/omnimam/backend/internal/apiserver/options"

// Config is the running configuration structure of the server.
type Config struct {
	*options.Options
}

// CreateConfigFromOptions creates a running configuration instance based
// on a given server command line or configuration file option.
func CreateConfigFromOptions(opts *options.Options) (*Config, error) {
	return &Config{opts}, nil
}
