package config

import (
	"fmt"
	"os"
	"path/filepath"
)

const AppName = "gogcli"

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}

	return filepath.Join(base, AppName), nil
}

func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure config dir: %w", err)
	}

	return dir, nil
}

// KeyringDir is where the keyring "file" backend stores encrypted entries.
//
// We keep this separate from the main config dir because the file backend creates
// one file per key.
func KeyringDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "keyring"), nil
}

func EnsureKeyringDir() (string, error) {
	dir, err := KeyringDir()
	if err != nil {
		return "", err
	}
	// keyring's file backend uses 0700 by default; match that.

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure keyring dir: %w", err)
	}

	return dir, nil
}

func ClientCredentialsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "credentials.json"), nil
}

func DriveDownloadsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "drive-downloads"), nil
}

func EnsureDriveDownloadsDir() (string, error) {
	dir, err := DriveDownloadsDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure drive downloads dir: %w", err)
	}

	return dir, nil
}

func GmailAttachmentsDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "gmail-attachments"), nil
}

func EnsureGmailAttachmentsDir() (string, error) {
	dir, err := GmailAttachmentsDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure gmail attachments dir: %w", err)
	}

	return dir, nil
}

func GmailWatchDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "state", "gmail-watch"), nil
}

func KeepServiceAccountPath(email string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, fmt.Sprintf("keep-sa-%s.json", email)), nil
}

func EnsureGmailWatchDir() (string, error) {
	dir, err := GmailWatchDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("ensure gmail watch dir: %w", err)
	}

	return dir, nil
}
