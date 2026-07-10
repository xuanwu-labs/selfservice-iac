package config

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("AETHER_DB_DSN", "postgres://aether:secret@localhost:5432/aether")
	t.Setenv("AETHER_HTTP_ADDR", ":9090")
	t.Setenv("AETHER_LOG_LEVEL", "debug")

	cfg, err := Load("nonexistent.yaml", "test")
	require.NoError(t, err)
	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, "debug", cfg.LogLevel)
	assert.Equal(t, "postgres://aether:secret@localhost:5432/aether", cfg.DB.DSN)
}

func TestLoadMissingDSN(t *testing.T) {
	// Ensure AETHER_DB_DSN is not set
	os.Unsetenv("AETHER_DB_DSN") //nolint:errcheck // best-effort cleanup in test

	_, err := Load("nonexistent.yaml", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AETHER_DB_DSN")
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("AETHER_DB_DSN", "postgres://aether:secret@localhost:5432/aether")
	// Don't set HTTP_ADDR or LOG_LEVEL — should use defaults

	cfg, err := Load("nonexistent.yaml", "test")
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, int32(10), cfg.DB.MaxConns)
}

func TestRedactDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "standard DSN",
			dsn:  "postgres://user:password123@db.example.com:5432/aether",
			want: "postgres://user:****@db.example.com:5432/aether",
		},
		{
			name: "no password",
			dsn:  "postgres://aether@localhost:5432/db",
			want: "postgres://aether@localhost:5432/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactDSN(tt.dsn)
			assert.Equal(t, tt.want, got)
			assert.False(t, strings.Contains(got, "password123"), "redacted DSN must not contain password")
		})
	}
}

func TestLogConfig(t *testing.T) {
	cfg := &Config{
		HTTPAddr: ":8080",
		LogLevel: "info",
		DB: DBConfig{
			DSN:      "postgres://aether:secret@localhost:5432/db",
			MaxConns: 10,
			Timeout:  "5s",
		},
	}

	// Should not panic
	Log(zap.NewNop(), cfg)
}
