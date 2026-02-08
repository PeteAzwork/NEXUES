package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all configuration for the Nexus service.
type Config struct {
	Port         int    `envconfig:"PORT" default:"8080"`
	MongoURI     string `envconfig:"MONGO_URI" default:"mongodb://mongo:27017"`
	DatabaseName string `envconfig:"DATABASE_NAME" default:"nexus"`
	Collection   string `envconfig:"COLLECTION_NAME" default:"snapshots"`
	LogLevel     string `envconfig:"LOG_LEVEL" default:"info"`
	ReadTimeout  int    `envconfig:"READ_TIMEOUT" default:"10"`
	WriteTimeout int    `envconfig:"WRITE_TIMEOUT" default:"10"`
	IdleTimeout  int    `envconfig:"IDLE_TIMEOUT" default:"120"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("NEXUS", &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}
	return &cfg, nil
}

func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.MongoURI == "" {
		return fmt.Errorf("mongo URI must not be empty")
	}
	if c.DatabaseName == "" {
		return fmt.Errorf("database name must not be empty")
	}
	if c.Collection == "" {
		return fmt.Errorf("collection name must not be empty")
	}
	return nil
}
