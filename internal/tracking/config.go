package tracking

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds tracking configuration
type Config struct {
	Enabled     bool   `json:"enabled"`
	WorkerURL   string `json:"worker_url"`
	TrackingKey string `json:"tracking_key"`
	AdminKey    string `json:"admin_key"`
}

// ConfigPath returns the path to the tracking config file
func ConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}

	return filepath.Join(configDir, "gog", "tracking.json"), nil
}

// LoadConfig loads tracking configuration from disk
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- path is derived from user config dir
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Enabled: false}, nil
		}

		return nil, fmt.Errorf("read tracking config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse tracking config: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves tracking configuration to disk
func SaveConfig(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if mkErr := os.MkdirAll(filepath.Dir(path), 0o700); mkErr != nil {
		return fmt.Errorf("create tracking config dir: %w", mkErr)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tracking config: %w", err)
	}

	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		return fmt.Errorf("write tracking config: %w", writeErr)
	}

	return nil
}

// IsConfigured returns true if tracking is set up
func (c *Config) IsConfigured() bool {
	return c.Enabled && c.WorkerURL != "" && c.TrackingKey != ""
}
