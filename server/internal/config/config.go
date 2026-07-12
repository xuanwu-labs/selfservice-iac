// Package config provides the Aether platform configuration loader.
//
// Loading priority (industry standard, 12-Factor / Spring Boot / viper):
//
//  1. Command-line flags (highest) — only infra: -config (file path), -env
//  2. Environment variables — AETHER_* (sensitive values like DB DSN)
//  3. Config file (YAML) — non-sensitive defaults, overridable per deployment
//  4. Built-in defaults (lowest)
//
// Sensitive values (passwords, secrets) can be set in yaml OR env vars.
// Env vars take priority (recommended for production). yaml is fine for
// local dev as long as the file is not committed with real secrets.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config holds all platform configuration.
// Modeled after ferret's Bootstrap, adapted for single-process Connect-RPC.
type Config struct {
	Service  ServiceConfig `mapstructure:"service"`
	Server   ServerConfig  `mapstructure:"server"`
	Data     DataConfig    `mapstructure:"data"`
	Auth     AuthConfig    `mapstructure:"auth"`
	LogLevel string        `mapstructure:"log_level"`
	Connect  ConnectConfig `mapstructure:"connect"`
	OTel     OTelConfig    `mapstructure:"otel"`
}

// ServiceConfig holds service identity (ferret: Service{Env, Name, Version}).
type ServiceConfig struct {
	Name    string `mapstructure:"name"`
	Version string `mapstructure:"version"`
	Env     string `mapstructure:"env"`
}

// ServerConfig holds HTTP server parameters (ferret: Server.HTTP{Network, Addr, Timeout}).
type ServerConfig struct {
	Addr    string `mapstructure:"addr"`
	Timeout string `mapstructure:"timeout"`
}

// DataConfig groups all infrastructure connection configs.
// Inspired by ferret's Data{Database, Redis}. Add future connections here.
type DataConfig struct {
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"` // optional; nil-safe if empty
}

// DatabaseConfig holds PostgreSQL connection parameters.
// Structured (not a flat DSN) so each field can be overridden via env or yaml.
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"ssl_mode"`
	MaxConns int32  `mapstructure:"max_conns"`
	Timeout  string `mapstructure:"timeout"`
}

// RedisConfig holds Redis connection parameters (ferret: Data_Redis).
// All fields optional — if Host is empty, Redis is considered disabled.
type RedisConfig struct {
	Network      string `mapstructure:"network"`       // "tcp" (default) or "unix"
	Addr         string `mapstructure:"addr"`          // "host:port"
	Password     string `mapstructure:"password"`      // from env var (sensitive)
	DB           int    `mapstructure:"db"`            // DB index (0-15)
	ReadTimeout  string `mapstructure:"read_timeout"`  // e.g. "3s"
	WriteTimeout string `mapstructure:"write_timeout"` // e.g. "3s"
}

// ConnectConfig controls the Connect-RPC layer.
type ConnectConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// AuthConfig holds JWT/OIDC authentication parameters.
// Inspired by ferret's Bootstrap{AuthKey, PublicKey}, structured for multi-issuer.
type AuthConfig struct {
	// JWT signing secret (HS256). From env var (sensitive).
	// Leave empty to disable JWT (e.g. dev mode behind gateway).
	JWTSecret string `mapstructure:"jwt_secret"`
	// JWT issuer claim (e.g. "aether-server").
	JWTIssuer string `mapstructure:"jwt_issuer"`
	// JWT token TTL (e.g. "24h").
	JWTTTL string `mapstructure:"jwt_ttl"`
	// OIDC issuer URLs for external IdP trust (D10/D10.1).
	// Each URL is an OIDC provider to verify tokens from.
	OIDCIssuers []string `mapstructure:"oidc_issuers"`
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
	v.SetDefault("service.name", "aether-server")
	v.SetDefault("service.version", "0.1.0")
	v.SetDefault("service.env", env)
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.timeout", "30s")
	v.SetDefault("log_level", "info")
	v.SetDefault("data.database.host", "localhost")
	v.SetDefault("data.database.port", 5432)
	v.SetDefault("data.database.ssl_mode", "disable")
	v.SetDefault("data.database.max_conns", 10)
	v.SetDefault("data.database.timeout", "5s")
	v.SetDefault("connect.enabled", true)
	v.SetDefault("auth.jwt_issuer", "aether-server")
	v.SetDefault("auth.jwt_ttl", "24h")
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

	// Explicit bindings for nested keys.
	_ = v.BindEnv("service.name")
	_ = v.BindEnv("service.version")
	_ = v.BindEnv("service.env")
	_ = v.BindEnv("server.addr")
	_ = v.BindEnv("server.timeout")
	_ = v.BindEnv("log_level")
	_ = v.BindEnv("data.database.host")
	_ = v.BindEnv("data.database.port")
	_ = v.BindEnv("data.database.user")
	_ = v.BindEnv("data.database.password")
	_ = v.BindEnv("data.database.database")
	_ = v.BindEnv("data.database.ssl_mode")
	_ = v.BindEnv("data.database.max_conns")
	_ = v.BindEnv("data.database.timeout")
	_ = v.BindEnv("data.redis.addr")
	_ = v.BindEnv("data.redis.password")
	_ = v.BindEnv("auth.jwt_secret")
	_ = v.BindEnv("auth.jwt_issuer")
	_ = v.BindEnv("auth.jwt_ttl")
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
	if cfg.Data.Database.Host == "" {
		return nil, fmt.Errorf("data.database.host is required (set via AETHER_DATA_DATABASE_HOST env var or config file)")
	}
	if cfg.Data.Database.User == "" {
		return nil, fmt.Errorf("data.database.user is required")
	}
	if cfg.Data.Database.Database == "" {
		return nil, fmt.Errorf("data.database.database (DB name) is required")
	}

	return &cfg, nil
}

// Log prints the config with sensitive fields redacted.
func Log(logger *zap.Logger, cfg *Config) {
	logger.Info("config loaded",
		zap.String("env", cfg.Service.Env),
		zap.String("service_name", cfg.Service.Name),
		zap.String("server_addr", cfg.Server.Addr),
		zap.String("log_level", cfg.LogLevel),
		zap.String("db.host", cfg.Data.Database.Host),
		zap.Int("db.port", cfg.Data.Database.Port),
		zap.String("db.name", cfg.Data.Database.Database),
		zap.Int32("db.max_conns", cfg.Data.Database.MaxConns),
		zap.String("redis.addr", cfg.Data.Redis.Addr),
		zap.String("auth.jwt_issuer", cfg.Auth.JWTIssuer),
		zap.String("auth.jwt_ttl", cfg.Auth.JWTTTL),
		zap.Bool("auth.jwt_configured", cfg.Auth.JWTSecret != ""),
		zap.Strings("auth.oidc_issuers", cfg.Auth.OIDCIssuers),
		zap.Bool("connect.enabled", cfg.Connect.Enabled),
		zap.String("otel.endpoint", cfg.OTel.Endpoint),
	)
}
