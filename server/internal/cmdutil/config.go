package cmdutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds CLI configuration (profiles, server URL, token).
// Stored at ~/.aether/config.json.
type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token,omitempty"`
	Profile   string `json:"profile,omitempty"`
}

// LoadConfig reads the CLI config from the given path.
// If the file doesn't exist, returns an empty Config (not an error).
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the CLI config to the given path.
func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// defaultConfigPath returns ~/.aether/config.json.
func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".aether/config.json"
	}
	return filepath.Join(home, ".aether", "config.json")
}
