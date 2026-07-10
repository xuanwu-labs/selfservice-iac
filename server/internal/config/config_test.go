package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("AETHER_DATA_DATABASE_HOST", "db.example.com")
	t.Setenv("AETHER_DATA_DATABASE_PORT", "5433")
	t.Setenv("AETHER_DATA_DATABASE_USER", "aether")
	t.Setenv("AETHER_DATA_DATABASE_PASSWORD", "secret")
	t.Setenv("AETHER_DATA_DATABASE_DATABASE", "aether_dev")
	t.Setenv("AETHER_SERVER_ADDR", ":9090")
	t.Setenv("AETHER_LOG_LEVEL", "debug")

	cfg, err := Load("nonexistent.yaml", "test")
	require.NoError(t, err)
	assert.Equal(t, ":9090", cfg.Server.Addr)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "db.example.com", cfg.Data.Database.Host)
	assert.Equal(t, 5433, cfg.Data.Database.Port)
}

func TestLoadMissingDB(t *testing.T) {
	// config.yaml in server/ has host=localhost but no user/password set,
	// so validation should fail on user (or host if yaml not found).
	_, err := Load("nonexistent.yaml", "test")
	assert.Error(t, err)
	// Could be "host" or "user" depending on whether config.yaml is found.
	assert.Contains(t, err.Error(), "data.database.")
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AETHER_DATA_DATABASE_HOST", "localhost")
	t.Setenv("AETHER_DATA_DATABASE_USER", "aether")
	t.Setenv("AETHER_DATA_DATABASE_DATABASE", "aether_dev")

	cfg, err := Load("nonexistent.yaml", "test")
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.Server.Addr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, int32(10), cfg.Data.Database.MaxConns)
	assert.Equal(t, "test", cfg.Service.Env)
	assert.Equal(t, "aether-server", cfg.Service.Name)
	assert.Equal(t, "postgres", cfg.Data.Database.Driver)
}

func TestLogConfig(t *testing.T) {
	cfg := &Config{
		Service:  ServiceConfig{Name: "aether", Env: "test"},
		Server:   ServerConfig{Addr: ":8080"},
		LogLevel: "info",
		Data: DataConfig{
			Database: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "aether",
				Database: "aether_dev",
			},
		},
	}
	Log(zap.NewNop(), cfg)
}
