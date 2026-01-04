package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	path, pathErr := ConfigPath()
	if pathErr != nil {
		t.Fatalf("ConfigPath: %v", pathErr)
	}

	base := filepath.Base(path)
	if base != "config.json" {
		t.Fatalf("unexpected config file: %q", base)
	}

	dirBase := filepath.Base(filepath.Dir(path))
	if dirBase != AppName {
		t.Fatalf("unexpected config dir: %q", filepath.Dir(path))
	}
}

func TestReadConfig_Missing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	cfg, readErr := ReadConfig()
	if readErr != nil {
		t.Fatalf("ReadConfig: %v", readErr)
	}

	backend := cfg.KeyringBackend
	if backend != "" {
		t.Fatalf("expected empty config, got %q", backend)
	}
}

func TestReadConfig_JSON5(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))

	path, pathErr := ConfigPath()
	if pathErr != nil {
		t.Fatalf("ConfigPath: %v", pathErr)
	}

	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o700)
	if mkdirErr != nil {
		t.Fatalf("mkdir: %v", mkdirErr)
	}

	data := `{
  // allow comments + trailing commas
  keyring_backend: "file",
}`

	writeErr := os.WriteFile(path, []byte(data), 0o600)
	if writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	cfg, readErr := ReadConfig()
	if readErr != nil {
		t.Fatalf("ReadConfig: %v", readErr)
	}

	if got := strings.TrimSpace(cfg.KeyringBackend); got != "file" {
		t.Fatalf("expected keyring_backend=file, got %q", got)
	}
}
