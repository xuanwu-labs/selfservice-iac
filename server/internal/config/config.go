// Package config provides the Aether platform configuration loader.
// Uses viper for multi-source merge (flag > env > file > default).
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config holds all platform configuration.
type Config struct {
	HTTPAddr string   `mapstructure:"http_addr"`
	LogLevel string   `mapstructure:"log_level"`
	DB       DBConfig `mapstructure:"db"`
}

// DBConfig holds PostgreSQL connection parameters.
type DBConfig struct {
	DSN      string `mapstructure:"dsn"`
	MaxConns int32  `mapstructure:"max_conns"`
	Timeout  string `mapstructure:"timeout"`
}

// Load reads configuration from env vars + defaults and returns a typed Config.
// In骨架 phase we only use env vars + defaults (no config file yet).
func Load() (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("http_addr", ":8080")
	v.SetDefault("log_level", "info")
	v.SetDefault("db.max_conns", 10)
	v.SetDefault("db.timeout", "5s")

	// Env var binding (AETHER_HTTP_ADDR, AETHER_DB_DSN, etc.)
	v.SetEnvPrefix("AETHER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for nested keys (viper's AutomaticEnv doesn't always
	// pick up nested keys on Unmarshal without explicit BindEnv)
	_ = v.BindEnv("http_addr")
	_ = v.BindEnv("log_level")
	_ = v.BindEnv("db.dsn")
	_ = v.BindEnv("db.max_conns")
	_ = v.BindEnv("db.timeout")

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Validate required fields
	if cfg.DB.DSN == "" {
		return nil, fmt.Errorf("AETHER_DB_DSN is required")
	}

	return &cfg, nil
}

// Log prints the config with sensitive fields redacted.
func Log(logger *zap.Logger, cfg *Config) {
	logger.Info("config loaded",
		zap.String("http_addr", cfg.HTTPAddr),
		zap.String("log_level", cfg.LogLevel),
		zap.Int32("db.max_conns", cfg.DB.MaxConns),
		zap.String("db.timeout", cfg.DB.Timeout),
		zap.String("db.dsn", redactDSN(cfg.DB.DSN)),
	)
}

// redactDSN masks the password in a PostgreSQL DSN for safe logging.
func redactDSN(dsn string) string {
	// Format: postgres://user:password@host:port/db
	// We need to mask only the password part (between first : after user and @)
	atIdx := strings.LastIndex(dsn, "@")
	if atIdx < 0 {
		return dsn
	}
	schemeEnd := strings.Index(dsn, "://")
	if schemeEnd < 0 {
		return dsn
	}
	userStart := schemeEnd + 3
	// Find the colon that separates user from password (must be between userStart and atIdx)
	colonIdx := strings.Index(dsn[userStart:atIdx], ":")
	if colonIdx < 0 {
		// No password — return as-is
		return dsn
	}
	pwdStart := userStart + colonIdx + 1
	return dsn[:pwdStart] + "****" + dsn[atIdx:]
}

// ParseTimeout parses the timeout string into a duration.
func (c *DBConfig) ParseTimeout() (time.Duration, error) {
	if c.Timeout == "" {
		return 5 * time.Second, nil
	}
	return time.ParseDuration(c.Timeout)
}
