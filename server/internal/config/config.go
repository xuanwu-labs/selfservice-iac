// Package config provides the Aether platform configuration loader.
//
// Loading priority (industry standard, 12-Factor / Spring Boot / viper):
//
//  1. Command-line flags (highest) — only infra: -config (file path), -env
//  2. Environment variables — AETHER_* (sensitive values like DB DSN)
//  3. Config file (YAML) — non-sensitive defaults, overridable per deployment
//  4. Built-in defaults (lowest)
//
// Sensitive values (DSN, passwords, tokens) MUST come from env vars, NOT yaml.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config holds all platform configuration.
type Config struct {
	// Env is the deployment environment (dev/staging/prod), from -env flag.
	Env      string        `mapstructure:"env"`
	HTTPAddr string        `mapstructure:"http_addr"`
	LogLevel string        `mapstructure:"log_level"`
	DB       DBConfig      `mapstructure:"db"`
	Connect  ConnectConfig `mapstructure:"connect"`
	OTel     OTelConfig    `mapstructure:"otel"`
}

// DBConfig holds PostgreSQL connection parameters.
type DBConfig struct {
	DSN      string `mapstructure:"dsn"`
	MaxConns int32  `mapstructure:"max_conns"`
	Timeout  string `mapstructure:"timeout"`
}

// ConnectConfig controls the Connect-RPC layer.
type ConnectConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// OTelConfig holds OpenTelemetry collector backend parameters.
type OTelConfig struct {
	Endpoint    string `mapstructure:"endpoint"`
	ServiceName string `mapstructure:"service_name"`
}

// Load reads configuration with four-layer priority:
// flags (passed as params) > env vars > yaml file > defaults.
//
// configPath is the yaml file path (from -config flag, default "config.yaml").
// env is the environment identifier (from -env flag, default "dev").
// If the yaml file doesn't exist, Load falls back to env vars + defaults.
func Load(configPath, env string) (*Config, error) {
	v := viper.New()

	// Layer 4: defaults (lowest)
	v.SetDefault("env", env)
	v.SetDefault("http_addr", ":8080")
	v.SetDefault("log_level", "info")
	v.SetDefault("db.max_conns", 10)
	v.SetDefault("db.timeout", "5s")
	v.SetDefault("connect.enabled", true)
	v.SetDefault("otel.endpoint", "")
	v.SetDefault("otel.service_name", "aether-server")

	// Layer 3: config file (yaml)
	// File not existing is OK — fall back to env + defaults (dev convenience).
	v.SetConfigFile(configPath)
	if err := v.ReadInConfig(); err != nil {
		// Silently skip if file doesn't exist; fail on parse errors.
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config file %s: %w", configPath, err)
		}
	}

	// Layer 2: env vars (override file)
	v.SetEnvPrefix("AETHER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit bindings for nested keys (viper's AutomaticEnv doesn't always
	// pick up nested keys on Unmarshal without explicit BindEnv).
	_ = v.BindEnv("http_addr")
	_ = v.BindEnv("log_level")
	_ = v.BindEnv("db.dsn")
	_ = v.BindEnv("db.max_conns")
	_ = v.BindEnv("db.timeout")
	_ = v.BindEnv("connect.enabled")
	_ = v.BindEnv("otel.endpoint")
	_ = v.BindEnv("otel.service_name")

	// Layer 1: flags (highest) — env is already set via SetDefault above,
	// and configPath is used to locate the file. Other flag overrides can
	// be added here if needed.

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// Validate required fields
	if cfg.DB.DSN == "" {
		return nil, fmt.Errorf("AETHER_DB_DSN is required (set via env var or config file)")
	}

	return &cfg, nil
}

// Log prints the config with sensitive fields redacted.
func Log(logger *zap.Logger, cfg *Config) {
	logger.Info("config loaded",
		zap.String("env", cfg.Env),
		zap.String("http_addr", cfg.HTTPAddr),
		zap.String("log_level", cfg.LogLevel),
		zap.Int32("db.max_conns", cfg.DB.MaxConns),
		zap.String("db.timeout", cfg.DB.Timeout),
		zap.String("db.dsn", redactDSN(cfg.DB.DSN)),
		zap.Bool("connect.enabled", cfg.Connect.Enabled),
		zap.String("otel.endpoint", cfg.OTel.Endpoint),
	)
}

// redactDSN masks the password in a PostgreSQL DSN for safe logging.
func redactDSN(dsn string) string {
	atIdx := strings.LastIndex(dsn, "@")
	if atIdx < 0 {
		return dsn
	}
	schemeEnd := strings.Index(dsn, "://")
	if schemeEnd < 0 {
		return dsn
	}
	userStart := schemeEnd + 3
	colonIdx := strings.Index(dsn[userStart:atIdx], ":")
	if colonIdx < 0 {
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
