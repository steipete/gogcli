package tracking

import (
	"path/filepath"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	// Use temp dir
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg-config"))

	cfg := &Config{
		Enabled:     true,
		WorkerURL:   "https://test.workers.dev",
		TrackingKey: "testkey123",
		AdminKey:    "adminkey456",
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if loaded.WorkerURL != cfg.WorkerURL {
		t.Errorf("WorkerURL mismatch: got %q, want %q", loaded.WorkerURL, cfg.WorkerURL)
	}

	if loaded.TrackingKey != cfg.TrackingKey {
		t.Errorf("TrackingKey mismatch: got %q, want %q", loaded.TrackingKey, cfg.TrackingKey)
	}

	if !loaded.IsConfigured() {
		t.Error("IsConfigured should return true")
	}
}

func TestLoadConfigMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg-config"))

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Enabled {
		t.Error("Expected Enabled to be false for missing config")
	}

	if cfg.IsConfigured() {
		t.Error("IsConfigured should return false for missing config")
	}
}
