package secrets

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/steipete/gogcli/internal/config"
)

func TestResolveKeyringBackendInfo_Default(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("GOG_KEYRING_BACKEND", "")

	info, err := ResolveKeyringBackendInfo()
	if err != nil {
		t.Fatalf("ResolveKeyringBackendInfo: %v", err)
	}

	if info.Value != "auto" {
		t.Fatalf("expected auto, got %q", info.Value)
	}

	if info.Source != keyringBackendSourceDefault {
		t.Fatalf("expected source default, got %q", info.Source)
	}
}

func TestResolveKeyringBackendInfo_Config(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("GOG_KEYRING_BACKEND", "")

	path, pathErr := config.ConfigPath()
	if pathErr != nil {
		t.Fatalf("ConfigPath: %v", pathErr)
	}

	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o700)
	if mkdirErr != nil {
		t.Fatalf("mkdir: %v", mkdirErr)
	}

	writeErr := os.WriteFile(path, []byte(`{ keyring_backend: "file" }`), 0o600)
	if writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	info, err := ResolveKeyringBackendInfo()
	if err != nil {
		t.Fatalf("ResolveKeyringBackendInfo: %v", err)
	}

	value := info.Value
	if value != "file" {
		t.Fatalf("expected file, got %q", value)
	}

	source := info.Source
	if source != keyringBackendSourceConfig {
		t.Fatalf("expected source config, got %q", source)
	}
}

func TestResolveKeyringBackendInfo_EnvOverridesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg-config"))
	t.Setenv("GOG_KEYRING_BACKEND", "keychain")

	path, pathErr := config.ConfigPath()
	if pathErr != nil {
		t.Fatalf("ConfigPath: %v", pathErr)
	}

	mkdirErr := os.MkdirAll(filepath.Dir(path), 0o700)
	if mkdirErr != nil {
		t.Fatalf("mkdir: %v", mkdirErr)
	}

	writeErr := os.WriteFile(path, []byte(`{ keyring_backend: "file" }`), 0o600)
	if writeErr != nil {
		t.Fatalf("write config: %v", writeErr)
	}

	info, err := ResolveKeyringBackendInfo()
	if err != nil {
		t.Fatalf("ResolveKeyringBackendInfo: %v", err)
	}

	value := info.Value
	if value != "keychain" {
		t.Fatalf("expected keychain, got %q", value)
	}

	source := info.Source
	if source != keyringBackendSourceEnv {
		t.Fatalf("expected source env, got %q", source)
	}
}

func TestAllowedBackends_Invalid(t *testing.T) {
	_, err := allowedBackends(KeyringBackendInfo{Value: "nope"})
	if err == nil {
		t.Fatalf("expected error")
	}

	if isInvalid := errors.Is(err, errInvalidKeyringBackend); !isInvalid {
		t.Fatalf("expected invalid backend error, got %v", err)
	}
}
